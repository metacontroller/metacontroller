TAG = 0.2

PKG        := k8s.io/metacontroller
API_GROUPS := metacontroller/v1alpha1

all: install

install: generated_files
	go install

image: generated_files
	docker build -t metacontroller/metacontroller:$(TAG) .

push: image
	docker push metacontroller/metacontroller:$(TAG)

# Code generators
# https://github.com/kubernetes/community/blob/master/contributors/devel/api_changes.md#generate-code

generated_files: deepcopy clientset lister informer

# requires deepcopy-gen tool (`go get k8s.io/code-generator/cmd/deepcopy-gen`)
deepcopy:
	@echo "+ Generating deepcopy funcs for $(API_GROUPS)"
	@deepcopy-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-file-base zz_generated.deepcopy

# requires client-gen tool (`go get k8s.io/code-generator/cmd/client-gen`)
clientset:
	@echo "+ Generating clientsets for $(API_GROUPS)"
	@client-gen \
		--fake-clientset=false \
		--input $(API_GROUPS) \
		--input-base $(PKG)/apis \
		--clientset-path $(PKG)/client/generated/clientset

# requires lister-gen tool (`go get k8s.io/code-generator/cmd/lister-gen`)
lister:
	@echo "+ Generating lister for $(API_GROUPS)"
	@lister-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-package $(PKG)/client/generated/lister

# requires informer-gen tool (`go get k8s.io/code-generator/cmd/informer-gen`)
informer:
	@echo "+ Generating informer for $(API_GROUPS)"
	@informer-gen \
		--input-dirs $(PKG)/apis/$(API_GROUPS) \
		--output-package $(PKG)/client/generated/informer \
		--versioned-clientset-package $(PKG)/client/generated/clientset/internalclientset \
		--listers-package $(PKG)/client/generated/lister
