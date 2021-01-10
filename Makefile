PWD := ${CURDIR}
PATH := $(PWD)/hack/bin:$(PATH)
TAG = dev

PKG        := metacontroller.io
API_GROUPS := metacontroller/v1alpha1

export GO111MODULE=on
export GOTESTSUM_FORMAT=pkgname

all: install

install: generated_files
	go install

vendor: 
	@go mod download
	@go mod tidy
	@go mod vendor

unit-test: test-setup
	pkgs="$$(go list ./... | grep -v '/test/integration/\|/examples/\|hack')" ; \
		go test -i $${pkgs} && \
		gotestsum $${pkgs}

integration-test: test-setup
	gotestsum ./test/integration/... -v -timeout 5m -args --logtostderr -v=1

test-setup: vendor
	./hack/setup.sh 

image: generated_files
	docker build -t metacontrollerio/metacontroller:$(TAG) .

push: image
	docker push metacontrollerio/metacontroller:$(TAG)

# Code generators
# https://github.com/kubernetes/community/blob/master/contributors/devel/api_changes.md#generate-code

generated_files: deepcopy clientset lister informer

# also builds vendored version of deepcopy-gen tool
deepcopy:
	@go install ./vendor/k8s.io/code-generator/cmd/deepcopy-gen
	@echo "+ Generating deepcopy funcs for $(API_GROUPS)"
	@deepcopy-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-file-base zz_generated.deepcopy

# also builds vendored version of client-gen tool
clientset:
	@go install ./vendor/k8s.io/code-generator/cmd/client-gen
	@echo "+ Generating clientsets for $(API_GROUPS)"
	@client-gen \
		--fake-clientset=false \
		--input $(API_GROUPS) \
		--input-base $(PKG)/apis \
		--clientset-path $(PKG)/client/generated/clientset

# also builds vendored version of lister-gen tool
lister:
	@go install ./vendor/k8s.io/code-generator/cmd/lister-gen
	@echo "+ Generating lister for $(API_GROUPS)"
	@lister-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-package $(PKG)/client/generated/lister

# also builds vendored version of informer-gen tool
informer:
	@go install ./vendor/k8s.io/code-generator/cmd/informer-gen
	@echo "+ Generating informer for $(API_GROUPS)"
	@informer-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-package $(PKG)/client/generated/informer \
		--versioned-clientset-package $(PKG)/client/generated/clientset/internalclientset \
		--listers-package $(PKG)/client/generated/lister
