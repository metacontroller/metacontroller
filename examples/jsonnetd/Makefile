image:
	go build
	docker build -t gcr.io/enisoc-kubernetes/jsonnetd .

push: image
	gcloud docker -- push gcr.io/enisoc-kubernetes/jsonnetd
