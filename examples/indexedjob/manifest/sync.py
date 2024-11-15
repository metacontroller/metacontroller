#!/usr/bin/env python

# Copyright 2017 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from http.server import BaseHTTPRequestHandler, HTTPServer
import io
import json
import copy
import re

def is_job_finished(job):
  for condition in job.get('status', {}).get('conditions', []):
    if (condition['type'] == 'Complete' or condition['type'] == 'Failed') and condition['status'] == 'True':
      return True
  return False

def get_index(base_name, name):
  m = re.match(r'^(.*)-(\d+)$', name)
  if m and m.group(1) == base_name:
    return int(m.group(2))
  return -1

def new_pod(job, index):
  pod = copy.deepcopy(job['spec']['template'])
  pod['apiVersion'] = 'v1'
  pod['kind'] = 'Pod'
  pod['metadata'] = pod.get('metadata', {})
  pod['metadata']['name'] = '%s-%d' % (job['metadata']['name'], index)

  # Add env var to every container.
  for container in pod['spec']['containers']:
    env = container.get('env', [])
    env.append({'name': 'JOB_INDEX', 'value': str(index)})
    container['env'] = env

  return pod

class Controller(BaseHTTPRequestHandler):
  def sync(self, job, children) -> dict:
    # Arrange observed Pods by index, and count by phase.
    observed_pods = {}
    (active, succeeded, failed) = (0, 0, 0)
    for pod_name, pod in children['Pod.v1'].items():
      pod_index = get_index(job['metadata']['name'], pod_name)
      if pod_index >= 0:
        phase = pod.get('status', {}).get('phase')
        if phase == 'Succeeded':
          succeeded += 1
        elif phase == 'Failed':
          failed += 1
        else:
          active += 1
        observed_pods[pod_index] = pod

    # If the job already finished (either completed or failed) at some point,
    # stop actively managing Pods since they might get deleted by Pod GC.
    # Just generate a desired state for any observed Pods and return status.
    if is_job_finished(job):
      return {
        'status': job['status'],
        'children': [new_pod(job, i) for i, pod in observed_pods.items()]
      }

    # Compute status based on what we observed, before building desired state.
    spec_completions = job['spec'].get('completions', 1)
    desired_status = {'active': active, 'succeeded': succeeded, 'failed': failed}
    desired_status['conditions'] = [{'type': 'Complete', 'status': str(succeeded == spec_completions)}]

    # Generate desired state for existing Pods.
    desired_pods = {}
    for pod_index, pod in observed_pods.items():
      desired_pods[pod_index] = new_pod(job, pod_index)

    # Create more Pods as needed.
    spec_parallelism = job['spec'].get('parallelism', 1)
    for pod_index in range(spec_completions):
      if pod_index not in desired_pods and active < spec_parallelism:
        desired_pods[pod_index] = new_pod(job, pod_index)
        active += 1

    return {'status': desired_status, 'children': list(desired_pods.values())}

  def do_POST(self):
    observed = json.loads(self.rfile.read(int(self.headers.get('content-length'))))
    desired = self.sync(observed['parent'], observed['children'])

    self.send_response(200)
    self.send_header('Content-type', 'application/json')
    self.end_headers()
    self.wfile.write(io.BytesIO(json.dumps(desired).encode('utf-8')).getvalue())


HTTPServer(('', 80), Controller).serve_forever()
