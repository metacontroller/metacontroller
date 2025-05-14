#!/bin/bash

set -e
set -u

GOTESTSUM_VERSION="1.12.2"

PWD="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

GOTESTSUM_OS="linux"
if [[ "$OSTYPE" == "darwin"* ]]; then
    GOTESTSUM_OS="darwin"
fi

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
        GOTESTSUM_ARCH="amd64"
    elif [[ "${uname_arch}" == "aarch64" || "${uname_arch}" == "arm64" ]] ; then
        GOTESTSUM_ARCH="arm64"
    else
        >&2 echo "Unknown CPU architecture, cannot install gotestsum"
        exit 1
    fi
    mkdir -p "${PWD}"/bin/
    curl -sSL "https://github.com/gotestyourself/gotestsum/releases/download/v${GOTESTSUM_VERSION}/gotestsum_${GOTESTSUM_VERSION}_${GOTESTSUM_OS}_${GOTESTSUM_ARCH}.tar.gz" | tar -xz -C "${PWD}"/bin gotestsum
    chmod +x "${PWD}"/bin/gotestsum
fi
