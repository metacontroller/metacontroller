---
title: Building
toc: false
classes: wide
---
The page describes how to build Metacontroller for yourself.

First, check out the code:

```sh
# If you're going to build locally, make sure to
# place the repo according to the Go import path:
#   $GOPATH/src/k8s.io/metacontroller
cd $GOPATH/src
git clone {{ site.repo_url }}.git k8s.io/metacontroller
cd k8s.io/metacontroller
```

## Docker Build

The main [Dockerfile]({{ site.repo_file }}/Dockerfile) can be used to build the
Metacontroller server without any dependencies on the local build environment
except for Docker 17.05+ (for multi-stage support):

```console
src/k8s.io/metacontroller$ docker build -t <yourtag> .
```

## Local Build

To build locally, you'll need Go 1.9+ as well as
[dep](https://github.com/golang/dep) (to download Go dependencies):

```sh
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
dep ensure
go install
```

## Generated Files

If you make changes to the [Metacontroller API types]({{ site.repo_dir }}/apis/metacontroller/),
you may need to update generated files before building:

```sh
go get -u k8s.io/code-generator/cmd/{lister,client,informer,deepcopy}-gen
make generated_files
```