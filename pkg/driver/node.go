package driver

import (
	"context"
	"fmt"
	"os"
	"time"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"github.com/shein/gcs-csi/pkg/flags"
	"github.com/shein/gcs-csi/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"google.golang.org/grpc"
        //pb "github.com/ofek/csi-gcs/pkg/pb"
	pb "github.com/shein/gcs-csi/pkg/csifuse-proxy/pb"
	"k8s.io/utils/mount"
)

func (driver *GCSDriver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.V(4).Infof("Method NodePublishVolume called with: %s", protosanitizer.StripSecrets(req))

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume Volume Capability must be provided")
	}

	if req.VolumeCapability.GetMount() == nil || req.VolumeCapability.GetBlock() != nil {
		return nil, status.Error(codes.InvalidArgument, "Only volumeMode Filesystem is supported")
	}

	// Default Options
	var options = map[string]string{
		"bucket":   req.GetVolumeId(),
	}

	// Merge Volume Context
	if req.VolumeContext != nil {
		options = flags.MergeFlags(options, req.VolumeContext)
	}

	//var clientOpt option.ClientOption
	keyFile := ""
	if len(req.Secrets) != 0 {
		// Retrieve Secret Key
		var err error
		keyFile, err = util.GetKey(req.Secrets, KeyStoragePath, req.VolumeContext["csi.storage.k8s.io/pod.uid"])
		if err != nil {
			return nil, err
		}
	}

	notMnt, err := driver.mounter.IsLikelyNotMountPoint(req.TargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(4).Infof("mkdir targetpath:%v",req.TargetPath)
			if err := os.MkdirAll(req.TargetPath, 0750); err != nil {
				klog.V(4).Infof("mkdir targetpath:%v err:%v",req.TargetPath,err)
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if !notMnt {
		klog.V(4).Infof("mkdir targetpath notMnt")
		return &csi.NodePublishVolumeResponse{}, nil
	}

	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	if keyFile != "" {
		hostKeyFile := "/var/lib/kubelet/plugins/gcs.csi.shein.dev/keys/" + req.VolumeContext["csi.storage.k8s.io/pod.uid"]
		mountOptions = append(mountOptions, fmt.Sprintf("--key-file=%s", hostKeyFile))
	}
	//mountOptions = append(mountOptions, flags.ExtraFlags(options)...)
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "-o=ro")
	}

	//mountArgs := []string{}
	//mountArgs = append(mountArgs, mountOptions...)
	//mountArgs = append(mountArgs, options[flags.FLAG_BUCKET], req.TargetPath)
	//klog.V(4).Infof("mountArgs : %v", mountArgs)
	mountOptions = append(mountOptions,options[flags.FLAG_BUCKET], req.TargetPath)
	//mntArgs := []string{"--mount=/proc/1/ns/mnt","--pid=/proc/1/ns/pid","gcsfuse"}
	//mntArgs = append(mntArgs,mountOptions...)

	klog.V(4).Infof("mntOptin: %v",mountOptions)

	/*
	mkArgs := []string{"--mount=/proc/1/ns/mnt","mkdir","-p",req.TargetPath}
	cmd = exec.Command("nsenter",mkArgs...)
	_, err = cmd.CombinedOutput()
        if err != nil {
                klog.V(4).Infof("mkdir targetPath error:%v",err)
                return nil, status.Error(codes.Internal, err.Error())
        }*/

	mntOptstr := strings.Join(mountOptions," ")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	klog.V(2).Infof("start connecting to csifuse proxy, mntCmd: gcsfuse  args: %s", mntOptstr)
	conn, err := grpc.DialContext(ctx, "unix:///csi/csifuse-proxy.sock", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		klog.Errorf("failed to connect to csifuse proxy: %v", err)
		return nil, status.Errorf(codes.Internal,"connect csifuse error:%v",err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			klog.Errorf("failed to close connection to csifuse proxy: %v", err)
		}
	}()

	mountClient := pb.NewProxyClient(conn)
	klog.V(2).Infof("begin to mount with csifuse proxy, gcsfuse %s", mntOptstr)
	resp, err := mountClient.CsiMountInHost(context.TODO(), &pb.MountRequest{MntCmd: "gcsfuse", MntArgs: mntOptstr})
	if err != nil {
		klog.Error("GRPC call csiproxy returned with an error:", err)
		return nil, status.Errorf(codes.Internal, "csiproxy mount error: %v, output: %s", err, string(resp.GetOutput()))
	}

	/*
	serviceName := fmt.Sprintf("gcsfuse-%s.service", req.GetVolumeId())
	serviceContent := fmt.Sprintf(`
[Unit]
Description=GCSFuse Mount for %s
After=network.target

[Service]
Type=forking
Environment=GOOGLE_APPLICATION_CREDENTIALS=/etc/workload-identity/cred.json
ExecStart=/usr/bin/gcsfuse %s
Restart=on-failure
RestartSec=5
OOMScoreAdjust=-999

[Install]
WantedBy=multi-user.target
`, req.GetVolumeId(),mntOptstr)

	servicePath := filepath.Join("/etc/systemd/system", serviceName)
	cmd := exec.Command("nsenter", "--mount=/proc/1/ns/mnt", "sh", "-c", fmt.Sprintf("echo '%s' > %s", serviceContent, servicePath))
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write service file: %v, output: %s", err, output)
	}

	cmd = exec.Command("nsenter", "--mount=/proc/1/ns/mnt", "systemctl", "daemon-reload")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, status.Errorf(codes.Internal, "systemctl daemon-reload failed: %v, output: %s", err, output)
	}

	cmd = exec.Command("nsenter", "--mount=/proc/1/ns/mnt", "systemctl", "start", serviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, status.Errorf(codes.Internal, "systemctl start failed: %v, output: %s", err, output)
	}*/


	if driver.deleteOrphanedPods {
		err = util.RegisterMount(
			ctx,
			req.VolumeId,
			req.TargetPath,
			driver.nodeName,
			req.VolumeContext["csi.storage.k8s.io/pod.namespace"],
			req.VolumeContext["csi.storage.k8s.io/pod.name"],
			options,
		)
		if err != nil {
			return nil, err
		}
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (driver *GCSDriver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (response *csi.NodeUnpublishVolumeResponse, err error) {
	klog.V(4).Infof("Method NodeUnpublishVolume called with: %s", protosanitizer.StripSecrets(req))

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	notMnt, err := driver.mounter.IsLikelyNotMountPoint(req.TargetPath)

	if err != nil {
		if os.IsNotExist(err) {
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}
		// This error happens when the node container is restarted and the connection is lost
		if strings.Contains(err.Error(), "transport endpoint is not connected") {
			notMnt = false
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if notMnt {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	/*
	serviceName := fmt.Sprintf("gcsfuse-%s.service", req.GetVolumeId())
	cmd := exec.Command("nsenter", "--mount=/proc/1/ns/mnt", "systemctl", "stop", serviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		klog.V(4).Infof("failed to stop service: %v, output: %s", err, output)
	}

	servicePath := filepath.Join("/etc/systemd/system", serviceName)
	cmd = exec.Command("nsenter", "--mount=/proc/1/ns/mnt", "rm", "-f", servicePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		klog.V(4).Infof("failed to remove service file: %v, output: %s", err, output)
	}*/

	err = mount.CleanupMountPoint(req.GetTargetPath(), driver.mounter, false)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	parts := strings.Split(req.TargetPath,"/")
	keyFile := KeyStoragePath+"/" + parts[5]
	err = os.Remove(keyFile)
	if err != nil {
		klog.V(4).Infof("failed to remove keyfile: %s, error:%v",keyFile,err)
	}

	if driver.deleteOrphanedPods {
		err = util.UnregisterMount(ctx, req.VolumeId, req.TargetPath, driver.nodeName)
		if err != nil {
			klog.Error(err)
		}
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (driver *GCSDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("Method NodeGetInfo called with: %s", protosanitizer.StripSecrets(req))

	return &csi.NodeGetInfoResponse{NodeId: driver.nodeName}, nil
}

func (driver *GCSDriver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Infof("Method NodeGetCapabilities called with: %s", protosanitizer.StripSecrets(req))

	return &csi.NodeGetCapabilitiesResponse{Capabilities: []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
				},
			},
		},
	}}, nil
}

func (driver *GCSDriver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(4).Infof("Method NodeStageVolume called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Errorf(codes.Unimplemented, "NodeStageVolume: not implemented by %s", driver.name)
}

func (driver *GCSDriver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(4).Infof("Method NodeUnstageVolume called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Errorf(codes.Unimplemented, "NodeUnstageVolume: not implemented by %s", driver.name)
}

func (driver *GCSDriver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(4).Infof("Method NodeGetVolumeStats called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Errorf(codes.Unimplemented, "NodeGetVolumeStats: not implemented by %s", driver.name)
}

func (driver *GCSDriver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	klog.V(4).Infof("Method NodeExpandVolume called with: %s", protosanitizer.StripSecrets(req))

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetVolumePath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume path missing in request")
	}

	notMnt, err := driver.mounter.IsLikelyNotMountPoint(req.GetVolumePath())

	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Error(codes.NotFound, "Targetpath not found")
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if notMnt {
		return nil, status.Error(codes.NotFound, "Volume not mounted")
	}

	return &csi.NodeExpandVolumeResponse{}, nil
}
