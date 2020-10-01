package domain

type K8sConfig struct {
	Enabled   bool
	InCluster bool
	Namespace string
	PodName   string
	PodIp     string
}
