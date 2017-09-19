/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
This sample initializer is inspired by https://github.com/HubSpot/kubernetes/pull/7.
Like that admission plugin, this initializer can be used to mitigate the effects of
the annotations below being removed in favor of corresponding fields in Kubernetes 1.7+
(see https://github.com/kubernetes/kubernetes/issues/48327).
The advantage of an initializer is that it can be deployed instantly,
even on a hosted cluster like GKE, without rebuilding or upgrading Kubernetes.

Note that this only works in Kubernetes v1.6.11+, v1.7.7+, or v1.8.0+,
because it modifies Pod fields that are normally immutable.
*/

const hostnameAnnotation = 'pod.beta.kubernetes.io/hostname';
const subdomainAnnotation = 'pod.beta.kubernetes.io/subdomain';

module.exports = async function (context) {
  let pod = context.request.body.object;

  try {
    if (pod.metadata && pod.metadata.annotations) {
      pod.spec.hostname = pod.spec.hostname || pod.metadata.annotations[hostnameAnnotation];
      pod.spec.subdomain = pod.spec.subdomain || pod.metadata.annotations[subdomainAnnotation];
    }
  } catch (e) {
    return {status: 500, body: e.stack};
  }

  return {status: 200, body: {object: pod}, headers: {'Content-Type': 'application/json'}};
};
