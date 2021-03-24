module metacontroller.io

// This denotes the minimum supported language version and
// should not include the patch version.
go 1.16

require (
	github.com/prometheus/client_golang v1.9.0
	k8s.io/apimachinery v0.17.17
	k8s.io/client-go v0.17.17
	k8s.io/component-base v0.17.17
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
)

replace (
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.17
	k8s.io/client-go => k8s.io/client-go v0.17.17
	k8s.io/component-base => k8s.io/component-base v0.17.17
)
