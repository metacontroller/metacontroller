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

const podTemplateAnnotation = 'bluegreendeployments.ctl.enisoc.com/pod-template-json';

var deepEqual = function (lhs, rhs) {
  if (typeof lhs === 'object' && typeof rhs === 'object') {
    for (let key in lhs) {
      if (!(key in rhs) || !deepEqual(lhs[key], rhs[key])) {
        return false;
      }
    }
    for (let key in rhs) {
      if (!(key in lhs)) {
        return false;
      }
    }
    return true;
  }
  return lhs === rhs;
};

var deepCopy = function (obj) {
  return obj ? JSON.parse(JSON.stringify(obj)) : null;
};

var podTemplateEqual = function (bgd, rs) {
  return deepEqual(bgd.spec.template, JSON.parse(rs.metadata.annotations[podTemplateAnnotation]));
};

var updateReplicaSet = function (bgd, rs) {
  let color = rs.metadata.labels.color;
  rs.apiVersion = 'extensions/v1beta1';
  rs.kind = 'ReplicaSet';
  rs.metadata.labels = deepCopy(bgd.spec.template.metadata.labels) || {};
  rs.metadata.labels.color = color;
  rs.metadata.annotations = rs.metadata.annotations || {};
  rs.metadata.annotations[podTemplateAnnotation] = JSON.stringify(bgd.spec.template);
  rs.spec.selector = deepCopy(bgd.spec.selector);
  rs.spec.selector.matchLabels = rs.spec.selector.matchLabels || {};
  rs.spec.selector.matchLabels.color = color;
  rs.spec.template = deepCopy(bgd.spec.template);
  rs.spec.template.metadata.labels.color = color;
  return rs;
}

var newReplicaSet = function (bgd, color) {
  return updateReplicaSet(bgd, {
    metadata: {
      name: `${bgd.metadata.name}-${color}`,
      labels: {color: color}
    },
    spec: {}
  });
};

var newService = function (bgd) {
  let service = deepCopy(bgd.spec.service);
  service.apiVersion = 'v1';
  service.kind = 'Service';
  service.spec.selector.color = 'blue';
  return service;
};

module.exports = async function (context) {
  let observed = context.request.body;
  let desired = {status: {}, children: []};

  try {
    let bgd = observed.parent;
    let observedRS = observed.children['ReplicaSet.extensions/v1beta1'];

    // Create or update the Service.
    let service = observed.children['Service.v1'][bgd.spec.service.metadata.name] || newService(bgd);
    let activeColor = service.spec.selector.color;
    desired.children.push(service);

    // Create or update the ReplicaSets.
    let blueRS = observedRS[`${bgd.metadata.name}-blue`] || newReplicaSet(bgd, 'blue');
    let greenRS = observedRS[`${bgd.metadata.name}-green`] || newReplicaSet(bgd, 'green');
    desired.children.push(blueRS, greenRS);

    // Is the active ReplicaSet based on the most up-to-date Pod template?
    let [activeRS, inactiveRS] = (activeColor === 'blue') ? [blueRS, greenRS] : [greenRS, blueRS];
    activeRS.spec.replicas = bgd.spec.replicas;
    if (podTemplateEqual(bgd, activeRS)) {
      // No rollout necessary. Scale down inactive.
      inactiveRS.spec.replicas = 0;
    } else if (podTemplateEqual(bgd, inactiveRS)) {
      // The inactive RS already matches. Scale it up.
      inactiveRS.spec.replicas = bgd.spec.replicas;
      // Is it ready to swap?
      if (inactiveRS.status && inactiveRS.status.availableReplicas === bgd.spec.replicas) {
        // Swap active/inactive RS.
        service.spec.selector.color = inactiveRS.metadata.labels.color;
      }
    } else {
      // Neither RS matches.
      if (inactiveRS.spec.replicas === 0 && inactiveRS.status && inactiveRS.status.replicas === 0) {
        // Start a new rollout.
        updateReplicaSet(bgd, inactiveRS);
        inactiveRS.spec.replicas = bgd.spec.replicas;
      } else {
        // Some other rollout was in progress. We need to cancel it and wait.
        inactiveRS.spec.replicas = 0;
      }
    }

    // Compute controller status.
    desired.status.activeColor = activeColor;
    desired.status.active = activeRS.status;
    desired.status.inactive = inactiveRS.status;
  } catch (e) {
    return {status: 500, body: e.stack};
  }

  return {status: 200, body: desired, headers: {'Content-Type': 'application/json'}};
};
