#!/bin/bash

set -e
set -u

PWD="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

if [[ -f ${PWD}/bin/gotestsum ]] ; then
    chmod +x "${PWD}"/bin/gotestsum
    echo ""
    echo "+++ gotestsum installed"
    echo ""
else
    echo ""
    echo "+++ downloading gotestsum"
    echo ""
    mkdir -p "${PWD}"/bin/
    curl -sSL "https://github.com/gotestyourself/gotestsum/releases/download/v0.6.0/gotestsum_0.6.0_linux_amd64.tar.gz" | tar -xz -C ${PWD}/bin gotestsum
    chmod +x "${PWD}"/bin/gotestsum
fi
