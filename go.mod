module metacontroller

// This denotes the minimum supported language version and
// should not include the patch version.
go 1.16

require (
	github.com/google/go-cmp v0.5.6
	github.com/prometheus/client_golang v1.10.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/component-base v0.21.1
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.2
	k8s.io/client-go => k8s.io/client-go v0.20.2
	k8s.io/component-base => k8s.io/component-base v0.20.2
)
