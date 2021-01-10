#!/bin/bash

set -e
set -u

if [[ -f ./hack/bin/gotestsum ]] ; then
    echo ""
    echo "+++ gotestsum installed"
    echo ""
else
    echo ""
    echo "+++ downloading gotestsum"
    echo ""
    curl -sSL "https://github.com/gotestyourself/gotestsum/releases/download/v0.6.0/gotestsum_0.6.0_linux_amd64.tar.gz" | tar -xz -C ./hack/bin gotestsum
    chmod +x ./hack/bin/gotestsum
fi