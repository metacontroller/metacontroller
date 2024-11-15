#!/bin/bash
# Load up all the images to kind cluster
CLUSTER="metacontroller"

docker pull nginx:1.26.2
docker pull metacontroller/nodejs-server:0.1
docker pull python:3.11
docker pull busybox
docker pull metacontroller/jsonnetd:0.1

kind load docker-image --name ${CLUSTER} nginx:1.26.2
kind load docker-image --name ${CLUSTER} metacontroller/nodejs-server:0.1
kind load docker-image --name ${CLUSTER} python:3.11
kind load docker-image --name ${CLUSTER} busybox
kind load docker-image --name ${CLUSTER} metacontroller/jsonnetd:0.1
