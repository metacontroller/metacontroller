#!/bin/bash
# Generated online by https://argbash.io/generate
# This script runs the smoke tests that check basic Metacontroller functionality
# by running through each example controller.
#
# * You should only run this in a test cluster.
# * You should already have Metacontroller installed in your test cluster.
# * You should have kubectl in your PATH and configured for the right cluster.

die()
{
	local _ret="${2:-1}"
	test "${_PRINT_HELP:-no}" = yes && print_help >&2
	echo "$1" >&2
	exit "${_ret}"
}


begins_with_short_option()
{
	local first_option all_short_options='ih'
	first_option="${1:0:1}"
	test "$all_short_options" = "${all_short_options/$first_option/}" && return 1 || return 0
}

# THE DEFAULTS INITIALIZATION - OPTIONALS
_arg_ignore=


print_help()
{
	printf '%s\n' "The general script's help msg"
	printf 'Usage: %s [-i|--ignore <arg>] [--crd_version <arg>] [-h|--help]\n' "$0"
	printf '\t%s\n' "-i, --ignore: Ignore directory (no default)"
	printf '\t%s\n' "-h, --help: Prints help"
}


parse_commandline()
{
	while test $# -gt 0
	do
		_key="$1"
		case "$_key" in
			-i|--ignore)
				test $# -lt 2 && die "Missing value for the optional argument '$_key'." 1
				_arg_ignore="$2"
				shift
				;;
			--ignore=*)
				_arg_ignore="${_key##--ignore=}"
				;;
			-i*)
				_arg_ignore="${_key##-i}"
				;;
			-h|--help)
				print_help
				exit 0
				;;
			-h*)
				print_help
				exit 0
				;;
			*)
				_PRINT_HELP=yes die "FATAL ERROR: Got an unexpected argument '$1'" 1
				;;
		esac
		shift
	done
}

parse_commandline "$@"

set -e

logdir=$(mktemp -d -t metacontroller.XXX)
echo "Logging test output to ${logdir}"

ignore_dirs=( "${_arg_ignore[@]/%/\/test.sh}" )

echo "Ignored directories: ${ignore_dirs}"

cleanup() {
  if [ -z ${CI_MODE+x} ];
  then
    rm -rf "${logdir}"
  else
    echo "Not removing logs in CI mode"
  fi
}

trap cleanup EXIT

for test in */test.sh; do
  if [[ "${ignore_dirs[*]}" =~ ${test} ]]; then
    echo "Skipping ${test}"
    continue
  fi
  echo -n "Running ${test}..."
  testDirectory="$(dirname "${test}")"
  mkdir "${logdir}/${testDirectory}"
  metacontrollerLogFile="${logdir}/${test}-metacontroller.txt"
  kubectl logs metacontroller-0 --follow -n metacontroller > "${metacontrollerLogFile}" &
  serverPID=$!
  testLogFile="${logdir}/${test}.txt"
  if ! (cd "$(dirname "${test}")" && timeout --signal=SIGTERM 5m ./test.sh v1 > "${testLogFile}" 2>&1); then
    echo "FAILED"
    cat "${testLogFile}"
    echo "Test ${test} failed!"
    cat "${metacontrollerLogFile}"
    exit 1
  fi
  kill "${serverPID}" || true
  rm "${metacontrollerLogFile}" || true
  rm "${testLogFile}" || true
  echo "PASSED"
done
