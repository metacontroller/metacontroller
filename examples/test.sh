#!/bin/bash

# This script runs the smoke tests that check basic Metacontroller functionality
# by running through each example controller.
#
# * You should only run this in a test cluster.
# * You should already have Metacontroller installed in your test cluster.
# * You should have kubectl in your PATH and configured for the right cluster.

set -e

logfile=$(mktemp)
echo "Logging test output to ${logfile}"

cleanup() {
  rm ${logfile}
}
trap cleanup EXIT

for test in */test.sh; do
  echo -n "Running ${test}..."
  if ! (cd "$(dirname "${test}")" && ./test.sh > ${logfile} 2>&1); then
    echo "FAILED"
    cat ${logfile}
    echo "Test ${test} failed!"
    exit 1
  fi
  echo "PASSED"
done
