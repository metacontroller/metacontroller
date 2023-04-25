PWD := ${CURDIR}
PATH := $(PWD)/test/integration/hack/bin:$(PATH)
TAG?= dev
ADDITIONAL_BUILD_ARGUMENTS?=""
DOCKERFILE?="Dockerfile"

PKG		:= metacontroller
API_GROUPS := metacontroller/v1alpha1

export GO111MODULE=on
export GOTESTSUM_FORMAT=pkgname

CODE_GENERATOR_VERSION="v0.25.9"

PKGS = $(shell go list ./... | grep -v '/test/integration/\|/examples/')
COVER_PKGS = $(shell echo ${PKGS} | tr " " ",")

all: install

.PHONY: build
build: generated_files
	DEBUG=$(DEBUG) goreleaser build --single-target --rm-dist --snapshot --output $(PWD)/metacontroller

.PHONY: build_debug
build_debug: DEBUG=true
build_debug: build

.PHONY: unit-test
unit-test: test-setup
	@gotestsum -- -race -coverpkg="${COVER_PKGS}" -coverprofile=test/integration/hack/tmp/unit-test-coverage.out ${PKGS}

.PHONY: integration-test
integration-test: test-setup
	@cd ./test/integration; \
 	gotestsum -- -coverpkg="${COVER_PKGS}" -coverprofile=hack/tmp/integration-test-coverage.out ./... -timeout 5m -parallel 1

.PHONY: test-setup
test-setup:
	./test/integration/hack/setup.sh; \
	mkdir -p ./test/integration/hack/tmp; \

.PHONY: image
image: build
	docker build -t localhost/metacontroller:$(TAG) -f $(DOCKERFILE) .

.PHONY: image_debug
image_debug: TAG=debug
image_debug: DOCKERFILE=Dockerfile.debug
image_debug: build_debug
image_debug: image


# CRD generation
.PHONY: generate_crds
generate_crds:
	@echo "+ Generating crds"
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@controller-gen +crd +paths="./pkg/apis/..." +output:crd:stdout > manifests/production/metacontroller-crds-v1.yaml

# Code generators
# https://github.com/kubernetes/community/blob/master/contributors/devel/api_changes.md#generate-code

.PHONY: generated_files
generated_files: deepcopy clientset lister informer

# also builds vendored version of deepcopy-gen tool
.PHONY: deepcopy
deepcopy:
	@go install k8s.io/code-generator/cmd/deepcopy-gen@"${CODE_GENERATOR_VERSION}"
	@echo "+ Generating deepcopy funcs for $(API_GROUPS)"
	@deepcopy-gen \
		--input-dirs $(PKG)/pkg/apis/$(API_GROUPS) \
		--output-base $(PWD)/.. \
		--go-header-file ./hack/boilerplate.go.txt \
		--output-file-base zz_generated.deepcopy

# also builds vendored version of client-gen tool
.PHONY: clientset
clientset:
	@go install k8s.io/code-generator/cmd/client-gen@"${CODE_GENERATOR_VERSION}"
	@echo "+ Generating clientsets for $(API_GROUPS)"
	@client-gen \
		--fake-clientset=false \
		--go-header-file ./hack/boilerplate.go.txt \
		--input $(API_GROUPS) \
		--input-base $(PKG)/pkg/apis \
		--output-base $(PWD)/.. \
		--clientset-path $(PKG)/pkg/client/generated/clientset

# also builds vendored version of lister-gen tool
.PHONY: lister
lister:
	@go install k8s.io/code-generator/cmd/lister-gen@"${CODE_GENERATOR_VERSION}"
	@echo "+ Generating lister for $(API_GROUPS)"
	@lister-gen \
		--input-dirs $(PKG)/pkg/apis/$(API_GROUPS) \
		--go-header-file ./hack/boilerplate.go.txt \
		--output-base $(PWD)/.. \
		--output-package $(PKG)/pkg/client/generated/lister

# also builds vendored version of informer-gen tool
.PHONY: informer
informer:
	@go install k8s.io/code-generator/cmd/informer-gen@"${CODE_GENERATOR_VERSION}"
	@echo "+ Generating informer for $(API_GROUPS)"
	@informer-gen \
		--input-dirs $(PKG)/pkg/apis/$(API_GROUPS) \
		--go-header-file ./hack/boilerplate.go.txt \
		--output-base $(PWD)/.. \
		--output-package $(PKG)/pkg/client/generated/informer \
		--versioned-clientset-package $(PKG)/pkg/client/generated/clientset/internalclientset \
		--listers-package $(PKG)/pkg/client/generated/lister
