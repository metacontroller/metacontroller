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
  return rs && deepEqual(bgd.spec.template, JSON.parse(rs.metadata.annotations[podTemplateAnnotation]));
};

var newReplicaSet = function (bgd, color, replicas, template) {
  let rs = {
    apiVersion: 'apps/v1',
    kind: 'ReplicaSet',
    metadata: {
      name: `${bgd.metadata.name}-${color}`,
      labels: deepCopy(template.metadata.labels),
      annotations: {}
    },
    spec: {
      replicas: replicas,
      minReadySeconds: bgd.spec.minReadySeconds,
      selector: deepCopy(bgd.spec.selector),
      template: deepCopy(template)
    }
  };

  rs.metadata.labels.color = color;
  rs.metadata.annotations[podTemplateAnnotation] = JSON.stringify(template);
  rs.spec.selector.matchLabels = rs.spec.selector.matchLabels || {};
  rs.spec.selector.matchLabels.color = color;
  rs.spec.template.metadata.labels.color = color;

  return rs;
};

var newService = function (bgd, color) {
  let service = deepCopy(bgd.spec.service);
  service.apiVersion = 'v1';
  service.kind = 'Service';
  service.spec.selector.color = color;
  return service;
};

module.exports = async function (context) {
  let observed = context.request.body;
  let desired = {status: {}, children: []};

  console.log('observed: ' + observed)

  try {
    let bgd = observed.parent;
    let observedRS = observed.children['ReplicaSet.apps/v1'];

    // Compute status from observed state.
    let service = observed.children['Service.v1'][bgd.spec.service.metadata.name];
    let activeColor = service ? service.spec.selector.color : 'blue';

    let blueRS = observedRS[`${bgd.metadata.name}-blue`];
    let greenRS = observedRS[`${bgd.metadata.name}-green`];
    let [activeRS, inactiveRS] = (activeColor === 'blue') ? [blueRS, greenRS] : [greenRS, blueRS];

    desired.status = {
      activeColor: activeColor,
      active: activeRS ? activeRS.status : {},
      inactive: inactiveRS ? inactiveRS.status : {}
    };

    // Decide next step for rollout.
    let activeReplicas = activeRS ? activeRS.spec.replicas : bgd.spec.replicas;
    let activeTemplate = activeRS ? JSON.parse(activeRS.metadata.annotations[podTemplateAnnotation]) : bgd.spec.template;
    let inactiveReplicas = inactiveRS ? inactiveRS.spec.replicas : 0;
    let inactiveTemplate = inactiveRS ? JSON.parse(inactiveRS.metadata.annotations[podTemplateAnnotation]) : bgd.spec.template;

    // Is the active ReplicaSet based on the most up-to-date Pod template?
    if (podTemplateEqual(bgd, activeRS)) {
      // No rollout necessary. Scale down inactive.
      inactiveReplicas = 0;
    } else if (podTemplateEqual(bgd, inactiveRS)) {
      // The inactive RS already matches. Scale it up.
      inactiveReplicas = bgd.spec.replicas;
      // Is it ready to swap?
      if (inactiveRS.status && inactiveRS.status.availableReplicas === bgd.spec.replicas) {
        // Swap active/inactive RS.
        activeColor = inactiveRS.metadata.labels.color;
        [activeReplicas, inactiveReplicas] = [inactiveReplicas, activeReplicas];
        [activeTemplate, inactiveTemplate] = [inactiveTemplate, activeTemplate];
      }
    } else {
      // Neither RS matches.
      if (inactiveRS && inactiveRS.spec.replicas === 0 && inactiveRS.status && inactiveRS.status.replicas === 0) {
        // Start a new rollout.
        inactiveReplicas = bgd.spec.replicas;
        inactiveTemplate = bgd.spec.template;
      } else {
        // Some other rollout was in progress. We need to cancel it and wait.
        inactiveReplicas = 0;
      }
    }

    // Generate desired children.
    desired.children = [
      newService(bgd, activeColor),
      newReplicaSet(bgd, activeColor, activeReplicas, activeTemplate),
      newReplicaSet(bgd, activeColor == 'blue' ? 'green' : 'blue', inactiveReplicas, inactiveTemplate)
    ];
  } catch (e) {
    return {status: 500, body: e.stack};
  }

  return {status: 200, body: desired, headers: {'Content-Type': 'application/json'}};
};
