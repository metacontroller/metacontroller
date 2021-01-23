PWD := ${CURDIR}
PATH := $(PWD)/hack/bin:$(PATH)
TAG = dev

PKG        := metacontroller.io
API_GROUPS := metacontroller/v1alpha1

export GO111MODULE=on
export GOTESTSUM_FORMAT=pkgname

CONTROLLER_GEN := go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go

all: install

install: generated_files
	go install

.PHONY: vendor
vendor: 
	@go mod download
	@go mod tidy
	@go mod vendor

.PHONY: unit-test
unit-test: test-setup
	pkgs="$$(go list ./... | grep -v '/test/integration/\|/examples/\|hack')" ; \
		go test -i $${pkgs} && \
		gotestsum $${pkgs}

.PHONY: integration-test
integration-test: test-setup
	gotestsum ./test/integration/... -v -timeout 5m -args --logtostderr -v=1

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
	@$(CONTROLLER_GEN) "crd:trivialVersions=true,crdVersions=v1beta1" rbac:roleName=manager-role paths="./apis/..." output:crd:artifacts:config=tmp/crds-v1beta1
	@cat tmp/crds-v1beta1/*.yaml > manifests/production/metacontroller-crds-v1beta1.yaml
	@$(CONTROLLER_GEN) "crd:trivialVersions=false,crdVersions=v1" rbac:roleName=manager-role paths="./apis/..." output:crd:artifacts:config=tmp/crds-v1
	@cat tmp/crds-v1/*.yaml > manifests/production/metacontroller-crds-v1.yaml

# Code generators
# https://github.com/kubernetes/community/blob/master/contributors/devel/api_changes.md#generate-code

.PHONY: generated_files
generated_files: deepcopy clientset lister informer

# also builds vendored version of deepcopy-gen tool
.PHONY: deepcopy
deepcopy:
	@go install ./vendor/k8s.io/code-generator/cmd/deepcopy-gen
	@echo "+ Generating deepcopy funcs for $(API_GROUPS)"
	@deepcopy-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-file-base zz_generated.deepcopy

# also builds vendored version of client-gen tool
.PHONY: clientset
clientset:
	@go install ./vendor/k8s.io/code-generator/cmd/client-gen
	@echo "+ Generating clientsets for $(API_GROUPS)"
	@client-gen \
		--fake-clientset=false \
		--input $(API_GROUPS) \
		--input-base $(PKG)/apis \
		--clientset-path $(PKG)/client/generated/clientset

# also builds vendored version of lister-gen tool
.PHONY: lister
lister:
	@go install ./vendor/k8s.io/code-generator/cmd/lister-gen
	@echo "+ Generating lister for $(API_GROUPS)"
	@lister-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-package $(PKG)/client/generated/lister

# also builds vendored version of informer-gen tool
.PHONY: informer
informer:
	@go install ./vendor/k8s.io/code-generator/cmd/informer-gen
	@echo "+ Generating informer for $(API_GROUPS)"
	@informer-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-package $(PKG)/client/generated/informer \
		--versioned-clientset-package $(PKG)/client/generated/clientset/internalclientset \
		--listers-package $(PKG)/client/generated/lister
