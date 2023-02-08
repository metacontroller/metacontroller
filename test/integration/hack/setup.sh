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
    uname_arch="$(uname -m)"
    if [[ "${uname_arch}" == "x86_64" ]] ; then
        gotestsum_arch="amd64"
    elif [[ "${uname_arch}" == "aarch64" ]] ; then
        gotestsum_arch="arm64"
    else
        >&2 echo "Unknown CPU architecture, cannot install gotestsum"
        exit 1
    fi
    mkdir -p "${PWD}"/bin/
    curl -sSL "https://github.com/gotestyourself/gotestsum/releases/download/v0.6.0/gotestsum_0.6.0_linux_${gotestsum_arch}.tar.gz" | tar -xz -C "${PWD}"/bin gotestsum
    chmod +x "${PWD}"/bin/gotestsum
fi
