#!/bin/bash
set -euxo pipefail

printf "%s" "$1" > /tmp/release-notes.md
goreleaser release --release-notes /tmp/release-notes.md --clean
