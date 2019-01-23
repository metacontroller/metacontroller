#!/bin/bash

set -e
set -u

# This script downloads etcd and Kubernetes binaries that are
# used as part of the integration test environment,
# and places them in hack/bin/.
#
# The integration test framework expects these binaries to be found in the PATH.

# This is the kube-apiserver version to test against.
KUBE_VERSION="${KUBE_VERSION:-v1.11.3}"
KUBERNETES_RELEASE_URL="${KUBERNETES_RELEASE_URL:-https://dl.k8s.io}"

# This should be the etcd version downloaded by kubernetes/hack/lib/etcd.sh
# as of the above Kubernetes version.
ETCD_VERSION="${ETCD_VERSION:-v3.2.18}"

mkdir -p hack/bin
cd hack/bin

# Download kubectl.
rm -f kubectl
wget "${KUBERNETES_RELEASE_URL}/${KUBE_VERSION}/bin/linux/amd64/kubectl"
chmod +x kubectl

# Download kube-apiserver.
rm -f kube-apiserver
wget "${KUBERNETES_RELEASE_URL}/${KUBE_VERSION}/bin/linux/amd64/kube-apiserver"
chmod +x kube-apiserver

# Download etcd.
rm -f etcd
basename="etcd-${ETCD_VERSION}-linux-amd64"
filename="${basename}.tar.gz"
url="https://github.com/coreos/etcd/releases/download/${ETCD_VERSION}/${filename}"
wget "${url}"
tar -zxf "${filename}"
mv "${basename}/etcd" etcd
rm -rf "${basename}" "${filename}"
