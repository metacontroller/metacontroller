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

var getConditionStatus = function (obj, conditionType) {
  if (obj && obj.status && obj.status.conditions) {
    for (let condition of obj.status.conditions) {
      if (condition['type'] === conditionType) {
        return condition.status;
      }
    }
  }
  return 'Unknown';
};

var isRunningAndReady = function (pod) {
  return pod && pod.status && pod.status.phase === 'Running' && !pod.metadata.deletionTimestamp &&
    getConditionStatus(pod, 'Ready') === 'True';
};

var getOrdinal = function (baseName, name) {
  let match = name.match(/^(.*)-(\d+)$/);
  if (match && match[1] === baseName) return parseInt(match[2], 10);
  return -1;
};

var newPod = function (catset, ordinal) {
  let pod = JSON.parse(JSON.stringify(catset.spec.template));
  let podName = `${catset.metadata.name}-${ordinal}`;
  pod.apiVersion = 'v1';
  pod.kind = 'Pod';
  pod.metadata.name = podName;
  pod.spec.hostname = podName;
  pod.spec.subdomain = catset.spec.serviceName;
  if (catset.spec.volumeClaimTemplates) {
    pod.spec.volumes = pod.spec.volumes || [];
    for (let pvc of catset.spec.volumeClaimTemplates) {
      pod.spec.volumes.push({
        name: pvc.metadata.name,
        persistentVolumeClaim: {claimName: `${pvc.metadata.name}-${podName}`}
      });
    }
  }
  return pod;
};

var newPVC = function (name, template) {
  let pvc = JSON.parse(JSON.stringify(template));
  pvc.apiVersion = 'v1';
  pvc.kind = 'PersistentVolumeClaim';
  pvc.metadata.name = name;
  return pvc;
};

module.exports = async function (context) {
  let observed = context.request.body;
  let desired = {status: {}, children: []};

  try {
    let catset = observed.parent;

    // Arrange observed Pods by ordinal.
    let observedPods = {};
    for (let pod of Object.values(observed.children['Pod.v1'])) {
      let ordinal = getOrdinal(catset.metadata.name, pod.metadata.name);
      if (ordinal >= 0) observedPods[ordinal] = pod;
    }

    if (observed.finalizing) {
      // If the parent is being deleted, scale down to zero replicas.
      catset.spec.replicas = 0;
      // Mark the finalizer as done if there are no more Pods.
      desired.finalized = (Object.keys(observedPods).length == 0);
    }

    // Compute controller status.
    for (var ready = 0; ready < catset.spec.replicas && isRunningAndReady(observedPods[ready]); ready++);
    desired.status = {replicas: Object.keys(observedPods).length, readyReplicas: ready};

    // Generate desired Pods. First generate desired state for all existing Pods.
    let desiredPods = {};
    for (let ordinal in observedPods) {
      desiredPods[ordinal] = newPod(catset, ordinal);
    }
    // Fill in one missing Pod if all lower ordinals are Ready.
    if (ready < catset.spec.replicas && !(ready in desiredPods)) {
      desiredPods[ready] = newPod(catset, ready);
    }
    // If all desired Pods are Ready, see if we need to scale down.
    if (ready === catset.spec.replicas) {
      let maxOrdinal = Math.max(...Object.keys(desiredPods));
      if (maxOrdinal >= catset.spec.replicas) {
        delete desiredPods[maxOrdinal];
      }
    }
    // List Pods in descending order, since that determines rolling update order.
    for (let ordinal of Object.keys(desiredPods).sort((a,b) => a-b).reverse()) {
      desired.children.push(desiredPods[ordinal]);
    }

    if (catset.spec.volumeClaimTemplates) {
      // Generate desired PVCs.
      let desiredPVCs = {};
      for (let template of catset.spec.volumeClaimTemplates) {
        let baseName = `${template.metadata.name}-${catset.metadata.name}`;
        for (let i = 0; i < catset.spec.replicas; i++) {
          desired.children.push(newPVC(`${baseName}-${i}`, template));
        }
        // Also generate a desired state for existing PVCs outside the range.
        // PVCs are retained after scale down, but are deleted with the CatSet.
        for (let pvc of Object.values(observed.children['PersistentVolumeClaim.v1'])) {
          if (pvc.metadata.name.startsWith(baseName)) {
            let ordinal = getOrdinal(baseName, pvc.metadata.name);
            if (ordinal >= catset.spec.replicas) desired.children.push(newPVC(pvc.metadata.name, template));
          }
        }
      }
      desired.children.push(...Object.values(desiredPVCs));
    }
  } catch (e) {
    return {status: 500, body: e.stack};
  }

  return {status: 200, body: desired, headers: {'Content-Type': 'application/json'}};
};
