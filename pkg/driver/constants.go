package driver

const (
	CSIDriverName   = "gcs.csi.shein.dev"
	BucketMountPath = "/var/lib/kubelet/pods"
	KeyStoragePath  = "/csi/keys"
	WIStoragePath = "/etc/workload-identity/cred.json"
	DefaultGid      = 63147
	DefaultDirMode  = 0775
	DefaultFileMode = 0664
)
