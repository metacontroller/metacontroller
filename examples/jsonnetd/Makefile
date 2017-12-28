tag = 0.1

image:
	go build
	docker build -t gcr.io/enisoc-kubernetes/jsonnetd:$(tag) .

push: image
	gcloud docker -- push gcr.io/enisoc-kubernetes/jsonnetd:$(tag)
