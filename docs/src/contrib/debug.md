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
* build docker image with command - `make image` - this will compile the binary and build the container image
* load image into cluster (i.e. `kind load docker-image metacontrollerio/metacontroller:dev` in kind)
* restart pod

### Debug build

Debug requires building go sources in special way, which is done with `make build_debug`; the following image
built with the `Dockerfile.debug` dockerfile will then add it to the debug Docker image:

* apply debug manifests - `kubectl apply -k manifests/debug`
* build debug binary and image - `make image_debug`
* load image into cluster (i.e. `kind load docker-image metacontrollerio/metacontroller:debug` in kind)
* restart pod
* on startup, `go` process will wait for debugger on port 40000
* port forward port 40000 from container into localhost, i.e. `kubectl port-forward metacontroller-0 40000:40000`
* attach `go` debugger to port 40000 on localhost
