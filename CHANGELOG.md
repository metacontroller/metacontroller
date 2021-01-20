## [1.2.1](https://github.com/metacontroller/metacontroller/compare/v1.2.0...v1.2.1) (2021-01-20)


### Bug Fixes

* **docker:** Disable `latest` tag for distroless variants ([8ce7a8d](https://github.com/metacontroller/metacontroller/commit/8ce7a8d9ada65358e9f371f30db0e25374a3a35c))

# [1.2.0](https://github.com/metacontroller/metacontroller/compare/v1.1.3...v1.2.0) (2021-01-20)


### Bug Fixes

* **deps:** Update go to 1.15.7 ([e9d7a22](https://github.com/metacontroller/metacontroller/commit/e9d7a2211c3be833fa8216724fe7ba16715c1985))
* **deps:** Update k8s.io packages to 0.17.17 ([6ff338e](https://github.com/metacontroller/metacontroller/commit/6ff338ec8c96ccf8a46456b07b5bbb86ed6e33b6))
* **deps:** Update prometheus/client_golang to v1.9.0 ([d288bce](https://github.com/metacontroller/metacontroller/commit/d288bceaed5f17073044caf85a3af52213513479))


### Features

* **#31:** Add distroless images, migrate to build action v2 ([bbd9715](https://github.com/metacontroller/metacontroller/commit/bbd9715b08968fa146082480ddcac52c0bb67d74)), closes [#31](https://github.com/metacontroller/metacontroller/issues/31)

## [1.1.3](https://github.com/metacontroller/metacontroller/compare/v1.1.2...v1.1.3) (2021-01-20)


### Bug Fixes

* **deps:** update alpine docker tag to v3.13.0 ([2f62ec1](https://github.com/metacontroller/metacontroller/commit/2f62ec1506f21026e31c4947b29bd12ac88dafaa))

## [1.1.2](https://github.com/metacontroller/metacontroller/compare/v1.1.1...v1.1.2) (2021-01-11)


### Bug Fixes

* **deps:** Update k8s.io packages to v0.17.16 ([2f11f21](https://github.com/metacontroller/metacontroller/commit/2f11f21b8faca344ff6a2ed041adfe3e238d49bd))

## [1.1.1](https://github.com/metacontroller/metacontroller/compare/v1.1.0...v1.1.1) (2020-12-19)


### Bug Fixes

* **deps:** Update alpine to 3.12.3 and go to 1.15.6 ([091f3b2](https://github.com/metacontroller/metacontroller/commit/091f3b2231ac3c6b481dd159183739fdcc31e7b3))

# [1.1.0](https://github.com/metacontroller/metacontroller/compare/v1.0.3...v1.1.0) (2020-12-14)


### Features

* **perf:** Add a flag to configure the number of workers to run ([3f07022](https://github.com/metacontroller/metacontroller/commit/3f070229327b735a532d114b06175d6f46d30e82))

## [1.0.3](https://github.com/metacontroller/metacontroller/compare/v1.0.2...v1.0.3) (2020-12-13)


### Bug Fixes

* **security:** Update vunerable openssl packages -  CVE-2020-1971 ([060a2d9](https://github.com/metacontroller/metacontroller/commit/060a2d9b178936e7ed535310525bbf6e68ac77dd))

## [1.0.2](https://github.com/metacontroller/metacontroller/compare/v1.0.1...v1.0.2) (2020-12-11)


### Bug Fixes

* **deps:** update alpine docker tag to v3.12.2 ([08a9d26](https://github.com/metacontroller/metacontroller/commit/08a9d260a6366ba0caa0e747cdb96b99d01be9b2))

## [1.0.1](https://github.com/metacontroller/metacontroller/compare/v1.0.0...v1.0.1) (2020-12-10)


### Bug Fixes

* **deps:** update k8s.io packages to v0.17.15 ([34a0c98](https://github.com/metacontroller/metacontroller/commit/34a0c98c03d4940c8abd18c85ddbcb6f876ea837))

# [1.0.0](https://github.com/metacontroller/metacontroller/compare/v0.4.5...v1.0.0) (2020-12-01)


### chore

* **api:** Update CRD api versions to v1 ([c38b399](https://github.com/metacontroller/metacontroller/commit/c38b39944b04fa88185786c4d3ecd8d2dd951753))


### BREAKING CHANGES

* **api:** Migrated CRD api version to 'apiextensions.k8s.io/v1' introduced in kubernetes 1.16. This now makes 1.16 the minimal supported kubernetes version

Signed-off-by: Filip Petkovski <filip.petkovski@personio.de>

## [0.4.5](https://github.com/metacontroller/metacontroller/compare/v0.4.4...v0.4.5) (2020-11-28)


### Bug Fixes

* **deps:** update k8s.io packages to v0.17.14 ([4be7525](https://github.com/metacontroller/metacontroller/commit/4be75251892b4fca3db91ba767865303991f5064))
