## [4.5.2](https://github.com/metacontroller/metacontroller/compare/v4.5.1...v4.5.2) (2022-09-14)


### Bug Fixes

* **deps, security:** Update golang.org/x/net to fix CVE-2022-27664 ([a0ddbf3](https://github.com/metacontroller/metacontroller/commit/a0ddbf32ed9bb5f8328bac47a5efb5ea8c51111b))

## [4.5.1](https://github.com/metacontroller/metacontroller/compare/v4.5.0...v4.5.1) (2022-09-14)


### Bug Fixes

* **deps:** Update github.com/google/go-cmp to v0.5.9 ([f0d7c9d](https://github.com/metacontroller/metacontroller/commit/f0d7c9dad6e2277755e387c03da9b601d79eda83))
* **deps:** Update k8s.io/klog/v2 to v2.80.1 ([6d17582](https://github.com/metacontroller/metacontroller/commit/6d17582fb1c725a42e2f497e5e48fe43f934630c))

# [4.5.0](https://github.com/metacontroller/metacontroller/compare/v4.4.0...v4.5.0) (2022-09-13)


### Features

* **webhooks:** Select json deserialization mode of response: 'loose' (default) or 'strict' ([99bca2f](https://github.com/metacontroller/metacontroller/commit/99bca2fbe1c5fa20fee016dffd5856761ee90cc3)), closes [#572](https://github.com/metacontroller/metacontroller/issues/572)

# [4.4.0](https://github.com/metacontroller/metacontroller/compare/v4.3.9...v4.4.0) (2022-09-08)


### Features

* **hooks:** Add versioning to hook API [#496](https://github.com/metacontroller/metacontroller/issues/496) ([6bb9690](https://github.com/metacontroller/metacontroller/commit/6bb96908bec27d753ad484ac6042737b6f2b7f0e))

## [4.3.9](https://github.com/metacontroller/metacontroller/compare/v4.3.8...v4.3.9) (2022-09-04)


### Bug Fixes

* **deps:** Update k8s.io/klog/v2 to v2.80.0 ([9b6ee29](https://github.com/metacontroller/metacontroller/commit/9b6ee29776979021090724214bcfcd3654ec6246))
* **deps:** Update sigs.k8s.io/controller-runtime to v0.13.0 ([52db9d0](https://github.com/metacontroller/metacontroller/commit/52db9d05fb7fd2a14d53fcf36c04c6baa62c0d7b))

## [4.3.8](https://github.com/metacontroller/metacontroller/compare/v4.3.7...v4.3.8) (2022-08-31)


### Bug Fixes

* **deps:** Update go.uber.org/zap to v1.23.0 ([039b78f](https://github.com/metacontroller/metacontroller/commit/039b78f90f915fc49d5b6559b2d03358778ddec5))
* **deps:** Update k8s.io packages to v0.25.0 ([6396cbb](https://github.com/metacontroller/metacontroller/commit/6396cbbce815e8c800117f4a224f07f0b1773ae6))
* **deps:** Update k8s.io/utils to v0.0.0-20220823124924-e9cbc92d1a73 ([c501bd9](https://github.com/metacontroller/metacontroller/commit/c501bd90e523b77f4843b8957c23bb902e647b79))

## [4.3.7](https://github.com/metacontroller/metacontroller/compare/v4.3.6...v4.3.7) (2022-08-18)


### Bug Fixes

* **deps:** Update kubernetes packages to v0.24.4 ([76787c2](https://github.com/metacontroller/metacontroller/commit/76787c2b11c9d7937c9577dfc4b4d4598443d331))

## [4.3.6](https://github.com/metacontroller/metacontroller/compare/v4.3.5...v4.3.6) (2022-08-18)


### Bug Fixes

* **helm:** Publish helm chart on release ([7695d50](https://github.com/metacontroller/metacontroller/commit/7695d504750eb2648acfe4afe9a4838fede699a1)), closes [#621](https://github.com/metacontroller/metacontroller/issues/621)

## [4.3.5](https://github.com/metacontroller/metacontroller/compare/v4.3.4...v4.3.5) (2022-08-16)


### Bug Fixes

* **deps:** Update k8s.io/kube-openapi to v0.0.0-20220803164354-a70c9af30aea ([6d52edd](https://github.com/metacontroller/metacontroller/commit/6d52edd6c2261439357fcc7d1a60b555acbb33ff))
* **deps:** Update k8s.io/utils to v0.0.0-20220812165043-ad590609e2e5 ([a2e7af5](https://github.com/metacontroller/metacontroller/commit/a2e7af5931adec2af8f1270e43dbe4b9c54e0655))
* **release:** Set wrapping to single quotes in release command ([3250c2e](https://github.com/metacontroller/metacontroller/commit/3250c2e3e768a77a505b9faf2a13362d5ba0be4d))
* **release:** Use version with `v` prefix in docker push ([a53b064](https://github.com/metacontroller/metacontroller/commit/a53b06440ab3efcd8c3042b80983f6a0e7858482)), closes [#611](https://github.com/metacontroller/metacontroller/issues/611)


### Reverts

* Revert "chore(release): [skip ci] 4.3.6" ([0a88efa](https://github.com/metacontroller/metacontroller/commit/0a88efa130826b8dded701c917458d65fbaff13c))
* Revert "chore(release): [skip ci] 4.3.5" ([64aac8e](https://github.com/metacontroller/metacontroller/commit/64aac8e022277b93e37a6f3dd93f51ed92140a14))

## [4.3.4](https://github.com/metacontroller/metacontroller/compare/v4.3.3...v4.3.4) (2022-08-10)


### Bug Fixes

* **release:** Fix dockehub repository url ([de7e293](https://github.com/metacontroller/metacontroller/commit/de7e293312dcdef58e4530e3666b33d1fb454c8e))

## [4.3.3](https://github.com/metacontroller/metacontroller/compare/v4.3.2...v4.3.3) (2022-08-10)


### Bug Fixes

* **deps:** Update go.uber.org/zap to v1.22.0 ([faa93f2](https://github.com/metacontroller/metacontroller/commit/faa93f2df53922ecc463c6cb4b98e42d3e15c4bc))
* **deps:** Update k8s.io/utils to v0.0.0-20220810061631-2e139fc3ae1e ([61d1f9a](https://github.com/metacontroller/metacontroller/commit/61d1f9ad4beef6d29472fcb8f90d8932aec7a2e8))

## [4.3.2](https://github.com/metacontroller/metacontroller/compare/v4.3.1...v4.3.2) (2022-08-10)


### Bug Fixes

* **deps:** Update golang to 1.19 ([b53b5de](https://github.com/metacontroller/metacontroller/commit/b53b5de0595a0219301b4e0babe146f8eabd33c7))

## [4.3.1](https://github.com/metacontroller/metacontroller/compare/v4.3.0...v4.3.1) (2022-08-10)


### Bug Fixes

* **deps:** update dependency alpine to v3.16.2 ([80e11a3](https://github.com/metacontroller/metacontroller/commit/80e11a37663b05806d0a92dcd6c09c33501f78e7))

# [4.3.0](https://github.com/metacontroller/metacontroller/compare/v4.2.5...v4.3.0) (2022-07-21)


### Features

* **webhooks:** add etag support ([4c06eb6](https://github.com/metacontroller/metacontroller/commit/4c06eb6264ffa48c54f6fabb71100ecee43565ac))

## [4.2.5](https://github.com/metacontroller/metacontroller/compare/v4.2.4...v4.2.5) (2022-07-21)


### Bug Fixes

* **security:** Add ReadHeaderTimeout to pprof server to mitigate G112 ([a11059f](https://github.com/metacontroller/metacontroller/commit/a11059fb48f3f896839739d13ea97a54b5ca4c01))

## [4.2.4](https://github.com/metacontroller/metacontroller/compare/v4.2.3...v4.2.4) (2022-07-21)


### Bug Fixes

* **deps:** update dependency alpine to v3.16.1 ([d82df9a](https://github.com/metacontroller/metacontroller/commit/d82df9a01fd1af53a0b00150de47832212799e06))

## [4.2.3](https://github.com/metacontroller/metacontroller/compare/v4.2.2...v4.2.3) (2022-07-14)


### Bug Fixes

* **deps:** update dependency golang to v1.18.4 ([8d74fd4](https://github.com/metacontroller/metacontroller/commit/8d74fd435bc6aa9d3e6f07e1697e91b3bf02f072))

## [4.2.2](https://github.com/metacontroller/metacontroller/compare/v4.2.1...v4.2.2) (2022-07-14)


### Bug Fixes

* **deps:** Update controller-runtime to v0.12.3 ([2f7e062](https://github.com/metacontroller/metacontroller/commit/2f7e062d3dc7a4ee16246d248cc8eaa3d65e820c))
* **deps:** Update k8s dependencies to v0.24.3 ([c911040](https://github.com/metacontroller/metacontroller/commit/c911040e516603925ac0bbb786edb5f8ff097197))
* **deps:** Update k8s.io/klog/v2 to v2.70.1 ([63f1388](https://github.com/metacontroller/metacontroller/commit/63f13880c14bd435c4d762517c44dd68d3d20dc6))
* **deps:** Update k8s.io/utils to v0.0.0-20220713171938-56c0de1e6f5e ([63f6d0b](https://github.com/metacontroller/metacontroller/commit/63f6d0b87566c5723be998a81ad4cca47c3a36de))
* **security:** Fix CVE-2022-1996 by updating k8s.io/kube-openapi to v0.0.0-20220627174259-011e075b9cb8 ([42eabbc](https://github.com/metacontroller/metacontroller/commit/42eabbc0c74657a8ad95517689c1043e6c6cc6a3))

## [4.2.1](https://github.com/metacontroller/metacontroller/compare/v4.2.0...v4.2.1) (2022-06-16)


### Bug Fixes

* **deps:** update dependency golang to v1.18.3 ([676078e](https://github.com/metacontroller/metacontroller/commit/676078e2b7e25ed0e72a929204437aa662b27e74))
* **deps:** Update k8s.io packages to v0.24.1 ([44b5406](https://github.com/metacontroller/metacontroller/commit/44b5406510c7740e2d4ddc75310985396de82dbb))
* **deps:** Update zgo.at/zcache to v1.2.0 ([4bc4c94](https://github.com/metacontroller/metacontroller/commit/4bc4c94f1e7aa87fb33b69b0532c9e5b4ffc6abd))

# [4.2.0](https://github.com/metacontroller/metacontroller/compare/v4.1.0...v4.2.0) (2022-05-27)


### Features

* **helm:** Add service to chart and prometheus examples ([60916a9](https://github.com/metacontroller/metacontroller/commit/60916a93fb883f973a08a925e370380327aa3ff9))

# [4.1.0](https://github.com/metacontroller/metacontroller/compare/v4.0.3...v4.1.0) (2022-05-26)


### Bug Fixes

* **deps:** Update prometheus/client_golang to v1.12.2 ([85affb4](https://github.com/metacontroller/metacontroller/commit/85affb4f50cd428c21598e1eb2667b7e8eb5d3d7))
* **update:** Update controller-runtime to v0.12.1 ([dbd4fd9](https://github.com/metacontroller/metacontroller/commit/dbd4fd9aabfdf3cad51947b23044be9b6c1019ef))


### Features

* **Dockerfile:** Run apline images as nonroot user ([6e633bd](https://github.com/metacontroller/metacontroller/commit/6e633bd2036273d06b15c43d6a0882918843f18e))

## [4.0.3](https://github.com/metacontroller/metacontroller/compare/v4.0.2...v4.0.3) (2022-05-25)


### Bug Fixes

* **deps:** update dependency alpine to v3.16.0 ([568f988](https://github.com/metacontroller/metacontroller/commit/568f98898cfd4687aa73c3ff4e4969aa3ec3e236))

## [4.0.2](https://github.com/metacontroller/metacontroller/compare/v4.0.1...v4.0.2) (2022-05-11)


### Bug Fixes

* **deps:** update dependency golang to v1.18.2 ([0ed47d2](https://github.com/metacontroller/metacontroller/commit/0ed47d24a5fe76730727977871044a780d2164d6))

## [4.0.1](https://github.com/metacontroller/metacontroller/compare/v4.0.0...v4.0.1) (2022-05-10)


### Bug Fixes

* **deps:** Update github.com/google/go-cmp to v0.5.8 ([8f81c66](https://github.com/metacontroller/metacontroller/commit/8f81c66f9927efb2cc47eed0b64e9f8e71f058df))
* **deps:** Update go-logr/logr to 1.2.3 ([89dff29](https://github.com/metacontroller/metacontroller/commit/89dff2983277935077efcf7c12830b36c39016bb))
* **deps:** Update k8s.io packages to v0.24.0 ([8ac00eb](https://github.com/metacontroller/metacontroller/commit/8ac00eb232ec861403056d621b9fe126d07b89c1))

# [4.0.0](https://github.com/metacontroller/metacontroller/compare/v3.0.2...v4.0.0) (2022-05-04)


### Bug Fixes

* Add dlv to debug dockerfile and expose command in helm chart ([1e2b611](https://github.com/metacontroller/metacontroller/commit/1e2b611f6f2e52200adee462295895631f6beea2))


### chore

* **helm:** Use commandArgs for all command arguments ([b78476e](https://github.com/metacontroller/metacontroller/commit/b78476ec91624c1f97fa5acb48b755949ab02f9f))


### BREAKING CHANGES

* **helm:** The following helm values are removed.
The equivalent command arguments can now be passed directly to the
`commandArgs` value.

- discoveryInterval
- cacheFlushInterval
- zap.logLevel
- zap.devel
- zap.encoder
- zap.stacktraceLevel

Signed-off-by: Mike Smith <10135646+mjsmith1028@users.noreply.github.com>

## [3.0.2](https://github.com/metacontroller/metacontroller/compare/v3.0.1...v3.0.2) (2022-04-28)


### Bug Fixes

* **deps:** update dependency golang to v1.18.1 ([62109ed](https://github.com/metacontroller/metacontroller/commit/62109ed4c5a98c254aa89c31be75cd8399cca80f))

## [3.0.1](https://github.com/metacontroller/metacontroller/compare/v3.0.0...v3.0.1) (2022-04-11)


### Bug Fixes

* **dynamic apply:** Add `path` key as candidate to list merging ([a1de874](https://github.com/metacontroller/metacontroller/commit/a1de874c9421a3d95d96a31e8b9a328b4421f09e)), closes [#443](https://github.com/metacontroller/metacontroller/issues/443)

# [3.0.0](https://github.com/metacontroller/metacontroller/compare/v2.6.1...v3.0.0) (2022-04-08)


### Code Refactoring

* Use controller-runtime to read crd's ([f0b0c98](https://github.com/metacontroller/metacontroller/commit/f0b0c98978fc8d527ab911ad1c1783fe4629cc40))


### BREAKING CHANGES

* Dropping support for kubernetes older than 1.16

## [2.6.1](https://github.com/metacontroller/metacontroller/compare/v2.6.0...v2.6.1) (2022-04-06)


### Bug Fixes

* **helm:** Change helm field zapLogLevel to zap.logLevel ([870c8aa](https://github.com/metacontroller/metacontroller/commit/870c8aab776adc76322c8070d1e89932a469f57a)), closes [#482](https://github.com/metacontroller/metacontroller/issues/482)
* **helm:** Fix indenting for pdb spec ([1bcfb8f](https://github.com/metacontroller/metacontroller/commit/1bcfb8f3a611617db9a67723a22c46a0b643d749))

# [2.6.0](https://github.com/metacontroller/metacontroller/compare/v2.5.1...v2.6.0) (2022-04-06)


### Features

* **helm:** implement pod disruption budget ([d467934](https://github.com/metacontroller/metacontroller/commit/d46793449ed1ad5c68ac58240e15df1c2eb1146a))

## [2.5.1](https://github.com/metacontroller/metacontroller/compare/v2.5.0...v2.5.1) (2022-04-05)


### Bug Fixes

* **deps:** Update controller-runtime to v0.11.2 ([b243732](https://github.com/metacontroller/metacontroller/commit/b243732bee248da792388d5fb3c57465f7e85763))
* **deps:** Update k8s.api to v0.23.5 ([e88bce6](https://github.com/metacontroller/metacontroller/commit/e88bce6018961f7d6f540da3b44ab8568f602331))
* **deps:** Update klog/v2 to v2.60.1 ([d40bc8b](https://github.com/metacontroller/metacontroller/commit/d40bc8bcdb195b4ca312202a7bcfb31bbe11ca57))
* **deps:** Update zcache to v1.1.0 ([4e89577](https://github.com/metacontroller/metacontroller/commit/4e89577d86b18aa7b8a96c3e714bf64ade4b6845))

# [2.5.0](https://github.com/metacontroller/metacontroller/compare/v2.4.1...v2.5.0) (2022-04-05)


### Bug Fixes

* **deps:** update dependency alpine to v3.15.4 ([28beef9](https://github.com/metacontroller/metacontroller/commit/28beef9f1444955503bf63ea3d1dfba079126efe))


### Features

* **helm:** [#471](https://github.com/metacontroller/metacontroller/issues/471) - Expose rules and aggregateRule in ClusterRole ([41a462e](https://github.com/metacontroller/metacontroller/commit/41a462eb9f2577a9a3b5e064530d7c9769a6b29f))

## [2.4.1](https://github.com/metacontroller/metacontroller/compare/v2.4.0...v2.4.1) (2022-03-23)


### Bug Fixes

* **deps:** update dependency alpine to v3.15.2 ([ce68114](https://github.com/metacontroller/metacontroller/commit/ce6811460cbf8dadf42ac765471687dbd1c946af))

# [2.4.0](https://github.com/metacontroller/metacontroller/compare/v2.3.2...v2.4.0) (2022-03-21)


### Features

* Add priorityClassName to helm chart ([a4c5c10](https://github.com/metacontroller/metacontroller/commit/a4c5c106a9a0ada95fda1abb7a393b77b1fff64c))

## [2.3.2](https://github.com/metacontroller/metacontroller/compare/v2.3.1...v2.3.2) (2022-03-17)


### Bug Fixes

* **deps:** update dependency alpine to v3.15.1 ([3a005ec](https://github.com/metacontroller/metacontroller/commit/3a005ecf6d03a02fe46fef1a4d83c40a5b262a2c))

## [2.3.1](https://github.com/metacontroller/metacontroller/compare/v2.3.0...v2.3.1) (2022-03-17)


### Bug Fixes

* **deps:** update dependency golang to v1.18.0 ([3c433eb](https://github.com/metacontroller/metacontroller/commit/3c433eba38cf1d2053850b68359fd7fc9e0a942b))

# [2.3.0](https://github.com/metacontroller/metacontroller/compare/v2.2.6...v2.3.0) (2022-03-08)


### Features

* Add leader election ([29563b2](https://github.com/metacontroller/metacontroller/commit/29563b248979da69fa4722611a14809333c21d87))

## [2.2.6](https://github.com/metacontroller/metacontroller/compare/v2.2.5...v2.2.6) (2022-03-08)


### Bug Fixes

* **deps:** update dependency golang to v1.17.8 ([1c9e884](https://github.com/metacontroller/metacontroller/commit/1c9e884eaaf981e131b821f9ac55aa8eb12e3560))

## [2.2.5](https://github.com/metacontroller/metacontroller/compare/v2.2.4...v2.2.5) (2022-02-21)


### Bug Fixes

* **deps:** Update controller-runtime to v0.11.1 ([c4e9058](https://github.com/metacontroller/metacontroller/commit/c4e905852573e897bb1c005519a702bc60a17546))

## [2.2.4](https://github.com/metacontroller/metacontroller/compare/v2.2.3...v2.2.4) (2022-02-14)


### Bug Fixes

* **deps:** Update github.com/go-logr/logr to v1.2.2 ([1cf5dc4](https://github.com/metacontroller/metacontroller/commit/1cf5dc41c1b45496543cf3e388e4046dcf36c5bd))
* **deps:** Update go.uber.org/zap to v1.21.0 ([466bbc3](https://github.com/metacontroller/metacontroller/commit/466bbc3e8f233a15b6ed0c3208885c0678290a3c))
* **deps:** Update k8s.io/utils to v0.0.0-20220210201930-3a6ce19ff2f9 ([6c12b98](https://github.com/metacontroller/metacontroller/commit/6c12b989dc1630ecbb7526ec84fe8f391d25c070))

## [2.2.3](https://github.com/metacontroller/metacontroller/compare/v2.2.2...v2.2.3) (2022-02-14)


### Bug Fixes

* **release:** Fix latest tag, to point to alpine image ([ce02f32](https://github.com/metacontroller/metacontroller/commit/ce02f32cb9921758bd610d9723616549dd778852))

## [2.2.2](https://github.com/metacontroller/metacontroller/compare/v2.2.1...v2.2.2) (2022-02-14)


### Bug Fixes

* **deps:** update dependency golang to v1.17.7 ([007aeeb](https://github.com/metacontroller/metacontroller/commit/007aeeb0a37b61190753cdb6eb1e915f270aacff))

## [2.2.1](https://github.com/metacontroller/metacontroller/compare/v2.2.0...v2.2.1) (2022-02-14)


### Bug Fixes

* **deps:** Update controller-runtime to v0.11.0 and k8s to v0.23.3 ([937cbf2](https://github.com/metacontroller/metacontroller/commit/937cbf2beda18ace13cff29975fe6bfd527e0f27))
* **deps:** Update github.com/google/go-cmp to v0.5.7 ([5fa1396](https://github.com/metacontroller/metacontroller/commit/5fa139641fd4c4d011d6f3f2a987be2bbdce2d04))
* **deps:** Update github.com/prometheus/client_golang to v1.12.1 ([0897f66](https://github.com/metacontroller/metacontroller/commit/0897f663eef9282a64fa03869a81e4235944a734))

# [2.2.0](https://github.com/metacontroller/metacontroller/compare/v2.1.3...v2.2.0) (2022-01-28)


### Features

* Add pprof to enable profiling ([1dbf3f6](https://github.com/metacontroller/metacontroller/commit/1dbf3f61881181df488870deadec6d6daad9dfb5))

## [2.1.3](https://github.com/metacontroller/metacontroller/compare/v2.1.2...v2.1.3) (2022-01-24)


### Bug Fixes

* **customize:** [#414](https://github.com/metacontroller/metacontroller/issues/414) - Use 'UID' as cache key to avoid collisions between objects in different namespaces ([38126d1](https://github.com/metacontroller/metacontroller/commit/38126d16e211a014b19ae1c2d7c96b753b878d1e))

## [2.1.2](https://github.com/metacontroller/metacontroller/compare/v2.1.1...v2.1.2) (2022-01-22)


### Bug Fixes

* change invalid log message when InPlace update strategy is used ([1ca006e](https://github.com/metacontroller/metacontroller/commit/1ca006eeb3f4ecaca6776de53079186c161d93f1))

## [2.1.1](https://github.com/metacontroller/metacontroller/compare/v2.1.0...v2.1.1) (2022-01-17)


### Bug Fixes

* **hooks:** [#383](https://github.com/metacontroller/metacontroller/issues/383) - Correct handling of nil arrays in responses ([2d916fd](https://github.com/metacontroller/metacontroller/commit/2d916fd1f08399f6e170d2d63d982766a85d3301))

# [2.1.0](https://github.com/metacontroller/metacontroller/compare/v2.0.19...v2.1.0) (2022-01-09)


### Bug Fixes

* **deps:** update golang docker tag to v1.17.6 ([bf0e583](https://github.com/metacontroller/metacontroller/commit/bf0e5836248e96251ae71ac02fc6a34cf0617840))


### Features

* Add K8s API communiction check on startup ([de00e67](https://github.com/metacontroller/metacontroller/commit/de00e672263a71a60e2843a53b1ef2604c18f72a))

## [2.0.19](https://github.com/metacontroller/metacontroller/compare/v2.0.18...v2.0.19) (2021-12-09)


### Bug Fixes

* **deps:** update golang docker tag to v1.17.5 ([9f2abd8](https://github.com/metacontroller/metacontroller/commit/9f2abd811debfc7c2267e2c996d417d6854d4cfb))

## [2.0.18](https://github.com/metacontroller/metacontroller/compare/v2.0.17...v2.0.18) (2021-12-07)


### Bug Fixes

* **deps:** Update controller-runtime to v0.10.3 ([195fde1](https://github.com/metacontroller/metacontroller/commit/195fde15e4c6d3ca2a1b073c12c96abad1060970))
* **deps:** update golang docker tag to v1.17.4 ([937f91d](https://github.com/metacontroller/metacontroller/commit/937f91dfa084e1ea62551cf68391960e8e82afea))
* **discovery:** Do not fail if missing a subset of resources during API discover ([6dce893](https://github.com/metacontroller/metacontroller/commit/6dce893657d0e25a0e9183d99247c5a814135e3f))

## [2.0.17](https://github.com/metacontroller/metacontroller/compare/v2.0.16...v2.0.17) (2021-11-25)


### Bug Fixes

* **deps:** Update rest of k8s.io dependencies to v0.22.4 ([f5a4a1d](https://github.com/metacontroller/metacontroller/commit/f5a4a1dd74bbd7b59165301ba8fd1f8e22dc44a4))

## [2.0.16](https://github.com/metacontroller/metacontroller/compare/v2.0.15...v2.0.16) (2021-11-25)


### Bug Fixes

* **deps:** update alpine docker tag to v3.15.0 ([dd1e402](https://github.com/metacontroller/metacontroller/commit/dd1e4024764d74e1fda47609c0761a0a32f3ee3f))

## [2.0.15](https://github.com/metacontroller/metacontroller/compare/v2.0.14...v2.0.15) (2021-11-25)


### Bug Fixes

* **controller:** Ignore 404/409 error responses ([5c983a4](https://github.com/metacontroller/metacontroller/commit/5c983a4aa3a79cd5255509ad602e6190ecc414f1))

## [2.0.14](https://github.com/metacontroller/metacontroller/compare/v2.0.13...v2.0.14) (2021-11-24)


### Bug Fixes

* **deps:** Update json-patch to v5.6.0, k8s.io to v0.22.4 and k8s.io/utils ([889f355](https://github.com/metacontroller/metacontroller/commit/889f355134c36be2b11f0b9a9e4aab33236e237b))

## [2.0.13](https://github.com/metacontroller/metacontroller/compare/v2.0.12...v2.0.13) (2021-11-24)


### Bug Fixes

* **deps:** Update alpine to 3.14.3 and golang to 1.17.3 ([44c6595](https://github.com/metacontroller/metacontroller/commit/44c65951026a11b727b49028aa6ebff4981a343e))

## [2.0.12](https://github.com/metacontroller/metacontroller/compare/v2.0.11...v2.0.12) (2021-09-26)


### Bug Fixes

* Add command line arguments parameterization to Helm chart ([2081bcf](https://github.com/metacontroller/metacontroller/commit/2081bcf89fe6310838e47d2186a3a937ff62dfe9))

## [2.0.11](https://github.com/metacontroller/metacontroller/compare/v2.0.10...v2.0.11) (2021-09-26)


### Bug Fixes

* **deps:** update github.com/nsf/jsondiff commit hash to 0e9c064 ([b9fe982](https://github.com/metacontroller/metacontroller/commit/b9fe982947fd57d35750da2fce6d4edd90bee76e))
* **deps:** update github.com/nsf/jsondiff commit hash to 1e845ec ([55983c3](https://github.com/metacontroller/metacontroller/commit/55983c360bee54049a821809a88802c9c2ceca52))

## [2.0.10](https://github.com/metacontroller/metacontroller/compare/v2.0.9...v2.0.10) (2021-09-15)


### Bug Fixes

* **deps:** update golang docker tag to v1.17.1 ([8214d11](https://github.com/metacontroller/metacontroller/commit/8214d118ef9986fe19b25d09119c37555e40602a))

## [2.0.9](https://github.com/metacontroller/metacontroller/compare/v2.0.8...v2.0.9) (2021-09-08)


### Bug Fixes

* **docker:** [#351](https://github.com/metacontroller/metacontroller/issues/351) - Add arm32 image ([9685bd6](https://github.com/metacontroller/metacontroller/commit/9685bd67db5eae54e88c6ac79c689477c241dc22))

## [2.0.8](https://github.com/metacontroller/metacontroller/compare/v2.0.7...v2.0.8) (2021-09-08)


### Bug Fixes

* **deps:** Update k8s.io to v0.22.1 ([5382cc9](https://github.com/metacontroller/metacontroller/commit/5382cc966d86c4cb578b7a2396007ff942f5f5a3))

## [2.0.7](https://github.com/metacontroller/metacontroller/compare/v2.0.6...v2.0.7) (2021-09-08)


### Bug Fixes

* Delete metacontroller-crds-v1beta1.yaml ([ed15539](https://github.com/metacontroller/metacontroller/commit/ed155391e250ac73fef44f32f76ec94788eb417e))

## [2.0.6](https://github.com/metacontroller/metacontroller/compare/v2.0.5...v2.0.6) (2021-09-07)


### Bug Fixes

* **deps:** update golang docker tag to v1.17.0 ([e8a572b](https://github.com/metacontroller/metacontroller/commit/e8a572b9db260b9b82ab5c63c424b15d7d1ed8f1))

## [2.0.5](https://github.com/metacontroller/metacontroller/compare/v2.0.4...v2.0.5) (2021-09-07)


### Bug Fixes

* **deps:** update alpine docker tag to v3.14.2 ([d9393bf](https://github.com/metacontroller/metacontroller/commit/d9393bf9080a8b5511d56bb0aab1c9f349a6cbe1))

## [2.0.4](https://github.com/metacontroller/metacontroller/compare/v2.0.3...v2.0.4) (2021-08-10)


### Bug Fixes

* **deps:** update alpine docker tag to v3.14.1 ([0599592](https://github.com/metacontroller/metacontroller/commit/0599592ae8c8991c3c3a3ccf1314a9a7f77d7252))
* **deps:** update golang docker tag to v1.16.7 ([26b916a](https://github.com/metacontroller/metacontroller/commit/26b916a4d2e54162b985eeb5cdfc956b9d575c6d))

## [2.0.3](https://github.com/metacontroller/metacontroller/compare/v2.0.2...v2.0.3) (2021-08-03)


### Bug Fixes

* **metrics:** Add http client metrics ([3d2391d](https://github.com/metacontroller/metacontroller/commit/3d2391d1740418cb313e8dc0eb9c28d69a013461))

## [2.0.2](https://github.com/metacontroller/metacontroller/compare/v2.0.1...v2.0.2) (2021-07-31)


### Bug Fixes

* **deps:** update alpine:3.14.0 docker digest to adab384 ([cfc6956](https://github.com/metacontroller/metacontroller/commit/cfc69560695bbbf6b28c9402951808138eddb4c8))

## [2.0.1](https://github.com/metacontroller/metacontroller/compare/v2.0.0...v2.0.1) (2021-07-29)


### Bug Fixes

* **deps:** Update controller-runtime to v0.9.5 and k8s.io/utils ([5bfcb90](https://github.com/metacontroller/metacontroller/commit/5bfcb905f1d100eb71f5ac32c8081879aa5fbbed))


### Performance Improvements

* **webhooks:** [#255](https://github.com/metacontroller/metacontroller/issues/255) - Create httpClient per controller instead ad-hoc creation ([a8f5c39](https://github.com/metacontroller/metacontroller/commit/a8f5c3993996ffb90ce275ebf926dbeabf7e82eb))

# [2.0.0](https://github.com/metacontroller/metacontroller/compare/v1.5.20...v2.0.0) (2021-07-22)


### Bug Fixes

* **deps:** Update controller-runtime to 0.9.3 and k8s packages to v0.21.3 ([5d06b06](https://github.com/metacontroller/metacontroller/commit/5d06b0662f4acecac5c65792a3ab2e3a75e0d91e))
* **deps:** Update k8s.io/klog/v2 to v2.10.0 ([47b107d](https://github.com/metacontroller/metacontroller/commit/47b107da7da3416bd700ca0569813e7e5b5af239))


### Features

* Remove deprecated --client-config-path - switched to --kubeconfig ([9cf558a](https://github.com/metacontroller/metacontroller/commit/9cf558ae12f4f65d82d4547d00c3ef2a2a4ab74c))
* Rename --debug-addr to --metrics-address ([86cda55](https://github.com/metacontroller/metacontroller/commit/86cda55ea5d332fe3539e6d12c2ba9b91bad85aa))
* **logging:** [#233](https://github.com/metacontroller/metacontroller/issues/233) - Allow logging in json format ([8f11b37](https://github.com/metacontroller/metacontroller/commit/8f11b37aaa4dae991b7f5860d183a204361745ab))


### BREAKING CHANGES

* Flag --client-config-path is removed in favour of
--kubeconfig

Signed-off-by: grzesuav <grzesuav@gmail.com>
* Flag --debug-addr was renamed to --metrics-address

Signed-off-by: grzesuav <grzesuav@gmail.com>
* **logging:** Removed klog flags - `-v`, `--logtostderr` etc. Added zap logger flags
instead:
- --zap-log-level
- --zap-devel
- --zap-encoder
- --zap-stacktrace-level
Please read documentation (User Guide/Configuration) and/or check
manifest changes to check which should be used.

Signed-off-by: grzesuav <grzesuav@gmail.com>

## [1.5.20](https://github.com/metacontroller/metacontroller/compare/v1.5.19...v1.5.20) (2021-07-13)


### Bug Fixes

* **deps:** update golang docker tag to v1.16.6 ([2210565](https://github.com/metacontroller/metacontroller/commit/221056565061e0a3b7c5c84f1d517bbc07e0e76f))

## [1.5.19](https://github.com/metacontroller/metacontroller/compare/v1.5.18...v1.5.19) (2021-06-25)


### Bug Fixes

* **deps:** Update controllrt-runtime to 0.9.2 & k8s.io to v0.21.2 ([4031180](https://github.com/metacontroller/metacontroller/commit/4031180ed0c416b727da8c84efb3ee5b50b01dbd))

## [1.5.18](https://github.com/metacontroller/metacontroller/compare/v1.5.17...v1.5.18) (2021-06-23)


### Bug Fixes

* **metrics:** Utilize controller-runtime  metrics server ([45b80c3](https://github.com/metacontroller/metacontroller/commit/45b80c35170863e7377a2960bb8dacac0893a186))

## [1.5.17](https://github.com/metacontroller/metacontroller/compare/v1.5.16...v1.5.17) (2021-06-23)


### Bug Fixes

* **metrics:** [#289](https://github.com/metacontroller/metacontroller/issues/289) - Wait on signal before exiting to fix http server with metrics ([f646a90](https://github.com/metacontroller/metacontroller/commit/f646a9078785aef40deda04db9e870805f89f026))

## [1.5.16](https://github.com/metacontroller/metacontroller/compare/v1.5.15...v1.5.16) (2021-06-16)


### Bug Fixes

* **deps:** update alpine docker tag to v3.14.0 ([a252fbb](https://github.com/metacontroller/metacontroller/commit/a252fbb271f6691373c21d4ce1aee3bb2f8c7894))

## [1.5.15](https://github.com/metacontroller/metacontroller/compare/v1.5.14...v1.5.15) (2021-06-11)


### Bug Fixes

* **deps:** update module github.com/prometheus/client_golang to v1.11.0 ([06ea53d](https://github.com/metacontroller/metacontroller/commit/06ea53db876f3888004c34beab1bacc19aa50cd3))
* **deps:** update module sigs.k8s.io/controller-runtime to v0.9.0 and k8s.io packages to v0.21.1 ([c07dcb3](https://github.com/metacontroller/metacontroller/commit/c07dcb3a50a7d1976dd85a294ec71b69843c4a24))

## [1.5.14](https://github.com/metacontroller/metacontroller/compare/v1.5.13...v1.5.14) (2021-06-11)


### Bug Fixes

* **deps:** update golang docker tag to v1.16.5 ([d6fca11](https://github.com/metacontroller/metacontroller/commit/d6fca11088663668dfe04b1a6bf8bcedf9f45cdb))

## [1.5.13](https://github.com/metacontroller/metacontroller/compare/v1.5.12...v1.5.13) (2021-06-10)


### Bug Fixes

* **deps:** Update k8s and controller-runtime dependencies ([256f20e](https://github.com/metacontroller/metacontroller/commit/256f20e2c5c3e86fcacf882d781ed2881cbe6f95))

## [1.5.12](https://github.com/metacontroller/metacontroller/compare/v1.5.11...v1.5.12) (2021-05-30)


### Bug Fixes

* **customize:** [#259](https://github.com/metacontroller/metacontroller/issues/259) - Add guard for customize cache against concurrent writes ([b765c17](https://github.com/metacontroller/metacontroller/commit/b765c1731dfe28917fd84aee35c3e8e175a3850f))
* **deps:** Update klog/v2 to v2.9.0 ([47aa552](https://github.com/metacontroller/metacontroller/commit/47aa552f6648a016ca69edf7527404f006032224))

## [1.5.11](https://github.com/metacontroller/metacontroller/compare/v1.5.10...v1.5.11) (2021-05-29)


### Bug Fixes

* **deps:** update k8s.io/utils commit hash to 6fdb442 ([f713086](https://github.com/metacontroller/metacontroller/commit/f713086f6c93f0873e3257e1bc035cb6f4900819))
* **security:** Update golang.org/x/crypto because of CVE-2020-29652 ([38c0c2f](https://github.com/metacontroller/metacontroller/commit/38c0c2fdb7e24345a9fde27ad005cd095a6e29fc))

## [1.5.10](https://github.com/metacontroller/metacontroller/compare/v1.5.9...v1.5.10) (2021-05-25)


### Bug Fixes

* **deps:** Update go-cmp v0.5.6 prometheus/client_golang v1.10.0 ([1748a91](https://github.com/metacontroller/metacontroller/commit/1748a9171b37dde7d527edc16b74cf0ba7f49cc5))

## [1.5.9](https://github.com/metacontroller/metacontroller/compare/v1.5.8...v1.5.9) (2021-05-25)


### Performance Improvements

* **k8sClient:** use a cache-based version of k8s client ([17f3dd2](https://github.com/metacontroller/metacontroller/commit/17f3dd259890f4942ba22347e3dbe6bd2c9eedd3))

## [1.5.8](https://github.com/metacontroller/metacontroller/compare/v1.5.7...v1.5.8) (2021-05-09)


### Bug Fixes

* **deps:** Update golang to 1.16.4 ([b7e8a33](https://github.com/metacontroller/metacontroller/commit/b7e8a33a376d38f02f924fc5e154b3a07130af1e))
* **deps:** Update golang.org/x/text dependency due to CVE-2020-14040 ([3e6ae6a](https://github.com/metacontroller/metacontroller/commit/3e6ae6a6bb83804f9efc25b59e2024a7e159ac09))

## [1.5.7](https://github.com/metacontroller/metacontroller/compare/v1.5.6...v1.5.7) (2021-04-27)


### Bug Fixes

* **ControllerRevision:** [#144](https://github.com/metacontroller/metacontroller/issues/144) - Fix ControllerRevision management ([d405959](https://github.com/metacontroller/metacontroller/commit/d4059597ebc222ef01e4156b00b04536fa97ca64))

## [1.5.6](https://github.com/metacontroller/metacontroller/compare/v1.5.5...v1.5.6) (2021-04-21)


### Bug Fixes

* **events:** Emit events for controller sync errors ([9b7258d](https://github.com/metacontroller/metacontroller/commit/9b7258d10fb72ab6e7f49d6cf9af4eaf8d538dc7))

## [1.5.5](https://github.com/metacontroller/metacontroller/compare/v1.5.4...v1.5.5) (2021-04-15)


### Bug Fixes

* **deps:** update alpine docker tag to v3.13.5 ([bef407b](https://github.com/metacontroller/metacontroller/commit/bef407b441d572c0dfb78e7b5cc1da0882a25327))

## [1.5.4](https://github.com/metacontroller/metacontroller/compare/v1.5.3...v1.5.4) (2021-04-14)


### Bug Fixes

* **composite controller:** [#68](https://github.com/metacontroller/metacontroller/issues/68) - Skip registration until parent resource ([71fd2df](https://github.com/metacontroller/metacontroller/commit/71fd2df5c5505750636b94a3c83459cb85970fda))

## [1.5.3](https://github.com/metacontroller/metacontroller/compare/v1.5.2...v1.5.3) (2021-04-09)


### Bug Fixes

* **deps:** update golang docker tag to v1.16.3 ([a5ab1fc](https://github.com/metacontroller/metacontroller/commit/a5ab1fcab058af2db3d203fe3afeb8371ceabc92))

## [1.5.2](https://github.com/metacontroller/metacontroller/compare/v1.5.1...v1.5.2) (2021-04-01)


### Bug Fixes

* **deps:** update golang docker tag to v1.16.2 ([53d66d4](https://github.com/metacontroller/metacontroller/commit/53d66d4aaff1818871346d244dece0a9876cf020))

## [1.5.1](https://github.com/metacontroller/metacontroller/compare/v1.5.0...v1.5.1) (2021-03-31)


### Bug Fixes

* **deps:** update alpine docker tag to v3.13.4 ([c3901c9](https://github.com/metacontroller/metacontroller/commit/c3901c92d3d7dee61707a0777967690c5b0dae77))

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
