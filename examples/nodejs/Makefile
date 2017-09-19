image:
	docker build -t gcr.io/enisoc-kubernetes/nodejs-server .

push: image
	gcloud docker -- push gcr.io/enisoc-kubernetes/nodejs-server
