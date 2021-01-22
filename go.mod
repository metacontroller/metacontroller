module metacontroller.io

// This denotes the minimum supported language version and
// should not include the patch version.
go 1.14

require (
	github.com/go-logr/logr v0.3.0 // indirect
	github.com/google/go-jsonnet v0.14.0
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/prometheus/client_golang v1.9.0
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	golang.org/x/tools v0.0.0-20201120155355-20be4ac4bd6e // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	k8s.io/api v0.17.17
	k8s.io/apiextensions-apiserver v0.17.17
	k8s.io/apimachinery v0.17.17
	k8s.io/client-go/v12/v12 v12.0.0
	k8s.io/code-generator v0.17.17
	k8s.io/component-base v0.17.17
	k8s.io/gengo v0.0.0-20201113003025-83324d819ded // indirect
	k8s.io/klog/v2/v2 v2.4.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f
	sigs.k8s.io/controller-tools v0.2.4
)

replace (
	k8s.io/api => k8s.io/api v0.17.17
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.17
	k8s.io/client-go => k8s.io/client-go v0.17.17
	k8s.io/code-generator => k8s.io/code-generator v0.17.17
)
