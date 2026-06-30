PWD := ${CURDIR}
PATH := $(PWD)/test/integration/hack/bin:$(PATH)
TAG?= dev
ADDITIONAL_BUILD_ARGUMENTS?=""
DOCKERFILE?="Dockerfile"

E2E_CLUSTER_NAME            ?= metacontroller-e2e
E2E_NODE_IMAGE              ?= kindest/node:v1.35.0
E2E_VARIANT                 ?= dev
E2E_KEEP_CLUSTER_ON_FAILURE ?=

PKG		:= metacontroller
API_GROUPS := metacontroller/v1alpha1

export GO111MODULE=on
export GOTESTSUM_FORMAT=pkgname

CODE_GENERATOR_VERSION="v0.35.0"

PKGS = $(shell go list ./... | grep -v '/test/integration/\|/examples/')
COVER_PKGS = $(shell echo ${PKGS} | tr " " ",")

all: install

.PHONY: build
build: generated_files
	DEBUG=$(DEBUG) goreleaser build --single-target --clean --snapshot --output $(PWD)/metacontroller

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
	./test/integration/hack/get-kube-binaries.sh; \
	mkdir -p ./test/integration/hack/tmp; \

.PHONY: image
image: build
	docker build -t localhost/metacontroller:$(TAG) -f $(DOCKERFILE) --build-arg TARGETPLATFORM=. .

.PHONY: image_debug
image_debug: TAG=debug
image_debug: DOCKERFILE=Dockerfile.debug
image_debug: build_debug
image_debug: image

# e2e-test spins up a throwaway kind cluster, installs the freshly built image,
# and runs the full example suite against it.
#
# The suite is run with stdin redirected from /dev/null: some example tests use
# `kubectl run -i`, which otherwise keeps the terminal's stdin open and hangs
# after the container exits. CI does not hit this because its stdin is already a
# closed pipe.
.PHONY: e2e-test
e2e-test: image
	@set -e; \
	trap 'code=$$?; \
	  if [ $$code -ne 0 ] && [ -n "$(E2E_KEEP_CLUSTER_ON_FAILURE)" ]; then \
	    echo "e2e failed (exit $$code): keeping cluster $(E2E_CLUSTER_NAME) for debugging"; \
	    kubectl --context kind-$(E2E_CLUSTER_NAME) logs metacontroller-0 -n metacontroller --tail=200 || true; \
	  else \
	    kind delete cluster --name $(E2E_CLUSTER_NAME) || true; \
	  fi; \
	  exit $$code' EXIT; \
	kind create cluster --name $(E2E_CLUSTER_NAME) --image "$(E2E_NODE_IMAGE)" --wait 120s; \
	kind load docker-image localhost/metacontroller:$(TAG) --name $(E2E_CLUSTER_NAME); \
	kubectl --context kind-$(E2E_CLUSTER_NAME) apply -k manifests/$(E2E_VARIANT); \
	kubectl --context kind-$(E2E_CLUSTER_NAME) rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller; \
	cd examples && CI_MODE=true ./test.sh </dev/null


# CRD generation
.PHONY: generate_crds
generate_crds:
	@echo "+ Generating crds"
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@controller-gen +crd +paths="./pkg/apis/..." +output:crd:stdout > manifests/production/metacontroller-crds-v1.yaml
	@cp manifests/production/metacontroller-crds-v1.yaml deploy/helm/metacontroller/crds/

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
		--go-header-file ./hack/boilerplate.go.txt \
		--output-file zz_generated.deepcopy.go \
		./pkg/apis/$(API_GROUPS)

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
		--output-dir $(PWD)/pkg/client/generated/clientset \
		--output-pkg $(PKG)/pkg/client/generated/clientset \
		--clientset-name internalclientset

# also builds vendored version of lister-gen tool
.PHONY: lister
lister:
	@go install k8s.io/code-generator/cmd/lister-gen@"${CODE_GENERATOR_VERSION}"
	@echo "+ Generating lister for $(API_GROUPS)"
	@lister-gen \
		--go-header-file ./hack/boilerplate.go.txt \
		--output-dir $(PWD)/pkg/client/generated/lister \
		--output-pkg $(PKG)/pkg/client/generated/lister \
		./pkg/apis/$(API_GROUPS)

# also builds vendored version of informer-gen tool
.PHONY: informer
informer:
	@go install k8s.io/code-generator/cmd/informer-gen@"${CODE_GENERATOR_VERSION}"
	@echo "+ Generating informer for $(API_GROUPS)"
	@informer-gen \
		--go-header-file ./hack/boilerplate.go.txt \
		--output-dir $(PWD)/pkg/client/generated/informer \
		--output-pkg $(PKG)/pkg/client/generated/informer \
		--versioned-clientset-package $(PKG)/pkg/client/generated/clientset/internalclientset \
		--listers-package $(PKG)/pkg/client/generated/lister \
		./pkg/apis/$(API_GROUPS)
