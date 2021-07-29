module metacontroller/test/integration

go 1.16

require (
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/utils v0.0.0-20210722164352-7f3ee0f31471
	metacontroller v0.0.0-00010101000000-000000000000
	sigs.k8s.io/controller-runtime v0.9.5
)

replace (
	k8s.io/api => k8s.io/api v0.21.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.3
	k8s.io/client-go => k8s.io/client-go v0.21.3
	metacontroller => ../..
)
