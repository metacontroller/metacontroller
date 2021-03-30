# [1.5.0](https://github.com/metacontroller/metacontroller/compare/v1.4.8...v1.5.0) (2021-03-30)


### Features

* [#170](https://github.com/metacontroller/metacontroller/issues/170) - Emit kubernetes events ([260acca](https://github.com/metacontroller/metacontroller/commit/260accaffe954c77614d14859ef4aba07c61bcc6))

## [1.4.8](https://github.com/metacontroller/metacontroller/compare/v1.4.7...v1.4.8) (2021-03-26)


### Bug Fixes

* **deps:** update alpine docker tag to v3.13.3 ([953ae99](https://github.com/metacontroller/metacontroller/commit/953ae99ed2e0e447025afc97c8500f7f271c8290))

## [1.4.7](https://github.com/metacontroller/metacontroller/compare/v1.4.6...v1.4.7) (2021-03-17)


### Bug Fixes

* **deps:** Update klog2 to v2.8.0 ([9cf1ecc](https://github.com/metacontroller/metacontroller/commit/9cf1ecc9b66636fcff74e8c4336341052620fa73))

## [1.4.6](https://github.com/metacontroller/metacontroller/compare/v1.4.5...v1.4.6) (2021-03-11)


### Bug Fixes

* **composite controller:** Fixed GroupVersion management ([c5f4c09](https://github.com/metacontroller/metacontroller/commit/c5f4c09a49f1c8192980e7c799d9d457c6dddb2c))

## [1.4.5](https://github.com/metacontroller/metacontroller/compare/v1.4.4...v1.4.5) (2021-02-21)


### Bug Fixes

* **release:** [#197](https://github.com/metacontroller/metacontroller/issues/197) - Change release message to trigger CI pipeline ([3fb3847](https://github.com/metacontroller/metacontroller/commit/3fb384787ebcf68ac3c777bdc9b9de4d4f0d60aa))

## [1.4.4](https://github.com/metacontroller/metacontroller/compare/v1.4.3...v1.4.4) (2021-02-19)


### Bug Fixes

* **deps:** update golang docker tag to v1.16.0 ([1a684bf](https://github.com/metacontroller/metacontroller/commit/1a684bf9db79e0efac8e3f7e849bf87357d39ccd))

## [1.4.3](https://github.com/metacontroller/metacontroller/compare/v1.4.2...v1.4.3) (2021-02-19)


### Bug Fixes

* **deps:** update alpine docker tag to v3.13.2 ([ecb8a13](https://github.com/metacontroller/metacontroller/commit/ecb8a1312163aa6bc889f77b24990711521283a5))

## [1.4.2](https://github.com/metacontroller/metacontroller/compare/v1.4.1...v1.4.2) (2021-02-05)


### Bug Fixes

* **deps:** update golang docker tag to v1.15.8 ([32c8b8f](https://github.com/metacontroller/metacontroller/commit/32c8b8f03afd9501c08b0063f43a9045a36d019a))

## [1.4.1](https://github.com/metacontroller/metacontroller/compare/v1.4.0...v1.4.1) (2021-02-05)


### Bug Fixes

* **deps:** Update klog2 to 2.5.0 ([2988b74](https://github.com/metacontroller/metacontroller/commit/2988b74142f7f378da9fa70e1e5b5421abc56494))
* Add build information ([00f9858](https://github.com/metacontroller/metacontroller/commit/00f9858b5013962b6c9737011c012c6e26ea1d6c))
* Update alpine to 3.13.1 ([7d10f84](https://github.com/metacontroller/metacontroller/commit/7d10f84609d51ce46a40eeedd3b0bb94e9b8edcd))

# [1.4.0](https://github.com/metacontroller/metacontroller/compare/v1.3.0...v1.4.0) (2021-01-25)


### Features

* Ship CRD's manifests also in version v1beta1 for kubernetes 1.15 ([284b3e2](https://github.com/metacontroller/metacontroller/commit/284b3e222bad1a54ceedc8efc4e1b4d308c82d63))

# [1.3.0](https://github.com/metacontroller/metacontroller/compare/v1.2.1...v1.3.0) (2021-01-22)


### Features

* **#69:** Migration of customize hook implementation ([7c959db](https://github.com/metacontroller/metacontroller/commit/7c959db081eab9f69340fcb23b46f7e5791c0321)), closes [#69](https://github.com/metacontroller/metacontroller/issues/69)
* Implement customize hook ([2facbdb](https://github.com/metacontroller/metacontroller/commit/2facbdbaa4f775670d5aab2959e41bd2dfc9e92e))

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
