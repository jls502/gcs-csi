package driver

const (
	CSIDriverName   = "gcs.csi.shein.dev"
	BucketMountPath = "/var/lib/kubelet/pods"
	KeyStoragePath  = "/csi/keys"
	DefaultGid      = 63147
	DefaultDirMode  = 0775
	DefaultFileMode = 0664
)
