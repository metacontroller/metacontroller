PWD := ${CURDIR}
ADDITIONAL_BUILD_ARGUMENTS?=""

PKG		:= metacontroller
API_GROUPS := metacontroller/v1alpha1

CODE_GENERATOR_VERSION="v0.24.3"

all: generate_crds

.PHONY: generate_crds
generate_crds:
	@echo "+ Generating crds"
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@controller-gen +crd:generateEmbeddedObjectMeta=true +paths="./api/..." +output:crd:stdout > v1/crdv1.yaml
