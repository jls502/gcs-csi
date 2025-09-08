package main

import (
    "context"
    "log"
    "net"
    "fmt"
    "flag"
    "os"
    "os/exec"
    "sync"
    "strings"

    "k8s.io/klog/v2"
    "google.golang.org/grpc"
    //pb "github.com/csi-proxy/pb"
    pb "github.com/shein/gcs-csi/pkg/csifuse-proxy/pb"
)

var (
    mutex sync.Mutex
    csifuseProxyPath = flag.String("csifuse-proxy-path", "/tmp/csifuse-proxy.sock", "csifuse-proxy path")
)

// 实现 GreeterServer 接口
type server struct {
    pb.UnimplementedProxyServer
}

func (s *server) CsiMountInHost(ctx context.Context, req *pb.MountRequest) (*pb.MountReply, error) {
    mutex.Lock()
    defer mutex.Unlock()

    log.Printf("recevi from client message: %v %v", req.MntCmd,req.MntArgs)

    var cmd *exec.Cmd
    cmd = exec.Command(req.MntCmd, strings.Split(req.MntArgs, " ")...)

    env := os.Environ()
    env = append(env, fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=/etc/workload-identity/cred.json"))
    cmd.Env = env

    output, err := cmd.CombinedOutput()
    if err != nil {
	klog.Error("gcsfuse mount failed: with error:", err.Error())
    } else {
	klog.V(2).Infof("successfully mounted")
    }

    return &pb.MountReply{Output: string(output)}, nil
}

func main() {
    klog.InitFlags(nil)
    _ = flag.Set("logtostderr", "true")
    flag.Parse()

    _ = os.Remove(*csifuseProxyPath)

    lis, err := net.Listen("unix", *csifuseProxyPath)
    if err != nil {
        klog.Error("failed to listen: %v", err)
    }
    s := grpc.NewServer()
    pb.RegisterProxyServer(s, &server{})
    klog.V(2).Infof("gRPC server listening on unix://%v",*csifuseProxyPath)
    if err := s.Serve(lis); err != nil {
        klog.Error("failed to serve: %v", err)
    }
}
