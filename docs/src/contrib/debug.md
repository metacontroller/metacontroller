# Local development and debugging

Tips and tricks for contributors

[[_TOC_]]

## Local run of metacontroller

There are different flavours of manifests shipped to help with local development:
- manifests/dev
- manifests/debug

### Development build

The main difference it that image defined in manifest is `metacontrollerio/metacontroller:dev`, therefore:
* apply dev manifests - `kubectl apply -k manifests/dev`
* build docker image with command - `docker build -t metacontrollerio/metacontroller:dev -f Dockerfile .`
* load image into cluster (i.e. `kind load docker-image metacontrollerio/metacontroller:dev` in kind)
* restart pod

### Debug build

Debug requires building go sources in special way, which is done in `Dockerfile.debug` dockerfile.
In order to use it, please:
* apply debug manifests - `kubectl apply -k manifests/debug`
* build debug image - `docker build -t metacontrollerio/metacontroller:debug -f Dockerfile.debug .`
* load image into cluster (i.e. `kind load docker-image metacontrollerio/metacontroller:debug` in kind)
* restart pod
* on startup, `go` process will wait for debugger on port 40000
* port forward port 40000 from container into localhost, i.e. `kubectl port-forward metacontroller-0 40000:40000`
* attach `go` debugger to port 40000 on localhost