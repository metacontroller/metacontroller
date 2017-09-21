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
    let observedPods = observed.children['Pod.v1'];
    let observedPVCs = observed.children['PersistentVolumeClaim.v1'];

    // Carry over observed PVCs. We never delete them as long as the parent controller exists.
    desired.children.push(...Object.values(observedPVCs));
    // Fill in any missing PVCs.
    if (catset.spec.volumeClaimTemplates) {
      for (let template of catset.spec.volumeClaimTemplates) {
        for (let i = 0; i < catset.spec.replicas; i++) {
          let name = `${template.metadata.name}-${catset.metadata.name}-${i}`;
          if (!(name in observedPVCs)) {
            desired.children.push(newPVC(name, template));
          }
        }
      }
    }

    // Arrange observed Pods by ordinal.
    let desiredPods = {};
    for (let podName in observedPods) {
      let ordinal = getOrdinal(catset.metadata.name, podName);
      if (ordinal >= 0) desiredPods[ordinal] = observedPods[podName];
    }
    // Fill in one missing Pod if all lower ordinals are Ready.
    for (var ready = 0; ready < catset.spec.replicas && isRunningAndReady(desiredPods[ready]); ready++);
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
    desired.children.push(...Object.values(desiredPods));

    // Compute controller status.
    desired.status = {replicas: Object.keys(observedPods).length, readyReplicas: ready};
  } catch (e) {
    return {status: 500, body: e.stack};
  }

  return {status: 200, body: desired, headers: {'Content-Type': 'application/json'}};
};
