module metacontroller.io/test/integration

go 1.15

require (
	k8s.io/api v0.17.17
	k8s.io/apiextensions-apiserver v0.17.17
	k8s.io/apimachinery v0.17.17
	k8s.io/client-go v0.17.17
	k8s.io/klog/v2 v2.5.0
	metacontroller.io v0.0.0-00010101000000-000000000000
)

replace (
	k8s.io/api => k8s.io/api v0.17.17
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.17
	k8s.io/client-go => k8s.io/client-go v0.17.17
	metacontroller.io => ../..
)
