PWD := ${CURDIR}
PATH := $(PWD)/hack/bin:$(PATH)
TAG?= dev
ADDITIONAL_BUILD_ARGUMENTS?=""

PKG        := metacontroller.io
API_GROUPS := metacontroller/v1alpha1

export GO111MODULE=on
export GOTESTSUM_FORMAT=pkgname

CODE_GENERATOR_VERSION="v0.17.17"

PKGS = $(shell go list ./... | grep -v '/test/integration/\|/examples/')
COVER_PKGS = $(shell echo ${PKGS} | tr " " ",")

all: install

.PHONY: install
install: generated_files
	go install -ldflags  "-X main.version=$(TAG)" $(ADDITIONAL_BUILD_ARGUMENTS)

.PHONY: vendor
vendor: 
	@go mod download
	@go mod tidy
	@go mod vendor

.PHONY: unit-test
unit-test: test-setup
	go test -i ${PKGS} && \
	gotestsum -- -coverpkg="${COVER_PKGS}" -coverprofile=tmp/unit-coverage.out ${PKGS}

.PHONY: integration-test
integration-test: test-setup
	gotestsum -- -coverpkg="${COVER_PKGS}" -coverprofile=tmp/integration-coverage.out ./test/integration/... -v -timeout 5m -args --logtostderr -v=1

.PHONY: test-setup
test-setup: vendor
	./hack/setup.sh 

.PHONY: image
image: generated_files
	docker build -t metacontrollerio/metacontroller:$(TAG) .



# CRD generation
# rember to remove unnesessary metadata fields and
# add "api-approved.kubernetes.io": "unapproved, request not yet submitted"
# to annotations
.PHONY: generate_crds
generate_crds:
	@echo "+ Generating crds"
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen
	@controller-gen "crd:trivialVersions=true,crdVersions=v1beta1" rbac:roleName=manager-role paths="./apis/..." output:crd:artifacts:config=tmp/crds-v1beta1
	@cat tmp/crds-v1beta1/*.yaml > manifests/production/metacontroller-crds-v1beta1.yaml
	@controller-gen "crd:trivialVersions=false,crdVersions=v1" rbac:roleName=manager-role paths="./apis/..." output:crd:artifacts:config=tmp/crds-v1
	@cat tmp/crds-v1/*.yaml > manifests/production/metacontroller-crds-v1.yaml

# Code generators
# https://github.com/kubernetes/community/blob/master/contributors/devel/api_changes.md#generate-code

.PHONY: generated_files
generated_files: deepcopy clientset lister informer

# also builds vendored version of deepcopy-gen tool
.PHONY: deepcopy
deepcopy:
	@go install k8s.io/code-generator/cmd/deepcopy-gen
	@echo "+ Generating deepcopy funcs for $(API_GROUPS)"
	@deepcopy-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--go-header-file ./hack/boilerplate.go.txt \
		--output-file-base zz_generated.deepcopy

# also builds vendored version of client-gen tool
.PHONY: clientset
clientset:
	@go install k8s.io/code-generator/cmd/client-gen
	@echo "+ Generating clientsets for $(API_GROUPS)"
	@client-gen \
		--fake-clientset=false \
		--go-header-file ./hack/boilerplate.go.txt \
		--input $(API_GROUPS) \
		--input-base $(PKG)/apis \
		--clientset-path $(PKG)/client/generated/clientset

# also builds vendored version of lister-gen tool
.PHONY: lister
lister:
	@go install k8s.io/code-generator/cmd/lister-gen
	@echo "+ Generating lister for $(API_GROUPS)"
	@lister-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--go-header-file ./hack/boilerplate.go.txt \
		--output-package $(PKG)/client/generated/lister

# also builds vendored version of informer-gen tool
.PHONY: informer
informer:
	@go install k8s.io/code-generator/cmd/informer-gen
	@echo "+ Generating informer for $(API_GROUPS)"
	@informer-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--go-header-file ./hack/boilerplate.go.txt \
		--output-package $(PKG)/client/generated/informer \
		--versioned-clientset-package $(PKG)/client/generated/clientset/internalclientset \
		--listers-package $(PKG)/client/generated/lister
