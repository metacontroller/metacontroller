tag = 0.1

build:
	go build

image: build
	docker build -t gcr.io/enisoc-kubernetes/jsonnetd:$(tag) .

push: image
	gcloud docker -- push gcr.io/enisoc-kubernetes/jsonnetd:$(tag)
