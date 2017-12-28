tag = 0.1

all: build

generated_files:
	deepcopy-gen -i ./apis/metacontroller/v1alpha1 -O zz_generated.deepcopy --bounding-dirs=k8s.io/metacontroller

build: generated_files
	go build -i
	go build

image: build
	docker build -t gcr.io/enisoc-kubernetes/metacontroller:$(tag) .

push: image
	gcloud docker -- push gcr.io/enisoc-kubernetes/metacontroller:$(tag)
