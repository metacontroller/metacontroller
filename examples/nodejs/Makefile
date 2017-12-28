tag = 0.1

image:
	docker build -t gcr.io/enisoc-kubernetes/nodejs-server:$(tag) .

push: image
	gcloud docker -- push gcr.io/enisoc-kubernetes/nodejs-server:$(tag)
