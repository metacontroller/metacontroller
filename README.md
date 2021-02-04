![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/metacontroller/metacontroller)
![GitHub Release Date](https://img.shields.io/github/release-date/metacontroller/metacontroller)
![GitHub](https://img.shields.io/github/license/metacontroller/metacontroller)
![Docker Image Size (latest semver)](https://img.shields.io/docker/image-size/metacontrollerio/metacontroller)
![Docker Pulls](https://img.shields.io/docker/pulls/metacontrollerio/metacontroller)
![GitHub contributors](https://img.shields.io/github/contributors/metacontroller/metacontroller)
[![Go Report Card](https://goreportcard.com/badge/github.com/metacontroller/metacontroller)](https://goreportcard.com/report/github.com/metacontroller/metacontroller)
[![codecov](https://codecov.io/gh/metacontroller/metacontroller/branch/master/graph/badge.svg?token=VU0L35J51Z)](https://codecov.io/gh/metacontroller/metacontroller)

# Metacontroller

Metacontroller is an add-on for Kubernetes that makes it easy to write and
deploy [custom controllers](https://kubernetes.io/docs/concepts/api-extension/custom-resources/#custom-controllers)
in the form of [simple scripts](https://metacontroller.github.io/metacontroller/).

## A New Home
This is the new community owned and actively maintained home for Metacontroller. The open-source [project](https://github.com/GoogleCloudPlatform/metacontroller) started by [GKE](https://cloud.google.com/kubernetes-engine/) is no longer maintained. All future updates and releases for Metacontroller will come from this repository. In time, all [issues](https://github.com/GoogleCloudPlatform/metacontroller/issues) from the previous repository will be triaged and moved [here](https://github.com/metacontroller/metacontroller/issues). We are excited to move forward with Metacontroller as a community maintained project. A big thank you to all of the wonderful Metacontroller community members that made this happen!

Following is the immediate plan of actions:
- [x] Make this repo same as https://github.com/GoogleCloudPlatform/metacontroller
- [x] Make a release that uploads the image [here](https://hub.docker.com/orgs/metacontrollerio)
- [x] Fix Docker image vulnerability [issue](https://github.com/GoogleCloudPlatform/metacontroller/issues/202)
- [ ] Merge changes from [Metac](https://github.com/AmitKumarDas/metac) to this repo

## Documentation

Please see the [documentation site](https://metacontroller.github.io/metacontroller/) for details
on how to install, use, or contribute to Metacontroller.

## Contact

Please file [GitHub issues](https://github.com/metacontroller/metacontroller/issues) for bugs, feature requests, and proposals.

Join the [#metacontroller](https://kubernetes.slack.com/messages/metacontroller/) channel on
[Kubernetes Slack](http://slack.kubernetes.io).


## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and the
[contributor guide](https://metacontroller.github.io/metacontroller/contrib.html).

## Licensing

This project is licensed under the [Apache License 2.0](LICENSE).
