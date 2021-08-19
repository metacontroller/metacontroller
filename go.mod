module metacontroller

// This denotes the minimum supported language version and
// should not include the patch version.
go 1.16

require (
	github.com/evanphx/json-patch/v5 v5.5.0
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.6
	github.com/nsf/jsondiff v0.0.0-20210303162244-6ea32392771e // test
	github.com/prometheus/client_golang v1.11.0
	go.uber.org/zap v1.18.1
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.21.3
	k8s.io/klog/v2 v2.10.0
	k8s.io/utils v0.0.0-20210722164352-7f3ee0f31471
	sigs.k8s.io/controller-runtime v0.9.5
	zgo.at/zcache v1.0.0
)

replace (
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.3
	k8s.io/client-go => k8s.io/client-go v0.21.3
	k8s.io/component-base => k8s.io/component-base v0.21.3
)
