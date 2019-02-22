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

from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer
import json
import copy
import re

def is_job_finished(job):
  if 'status' in job:
    desiredNumberScheduled = job['status'].get('desiredNumberScheduled',1)
    numberReady = job['status'].get('numberReady',0)
    if desiredNumberScheduled == numberReady and desiredNumberScheduled > 0:
      return True
  return False

def new_daemon(job):
  daemon = {}
  daemon = copy.deepcopy(job)
  daemon['apiVersion'] = 'apps/v1'
  daemon['kind'] = 'DaemonSet'
  daemon['metadata'] = {}
  daemon['metadata']['name'] = '%s-dj' % (job['metadata']['name'])
  daemon['metadata']['labels'] = copy.deepcopy(job['spec']['template']['metadata']['labels'])
  daemon['spec'] = {}
  daemon['spec']['template'] = copy.deepcopy(job['spec']['template'])
  del daemon['spec']['template']['spec']['containers']
  daemon['spec']['template']['spec']['initContainers'] = copy.deepcopy(job['spec']['template']['spec']['containers'])
  daemon['spec']['template']['spec']['containers'] = [{'name': "pause", 'image': job['spec'].get('pauseImage', 'gcr.io/google_containers/pause')}]
  daemon['spec']['selector'] = {'matchLabels': copy.deepcopy(job['spec']['template']['metadata']['labels'])}

  return daemon

class Controller(BaseHTTPRequestHandler):
  def sync(self, job, children):
    desired_status = {}
    desired_childs = {}
    child = '%s-dj' % (job['metadata']['name'])

    # self.log_message(" Children: %s", children)

    # If the job already finished (either completed or failed) at some point,
    # stop actively managing childs since they might get deleted by child GC.
    # Just generate a desired state for any observed childs and return status.
    if is_job_finished(job):
      desired_status = copy.deepcopy(job['status'])
      desired_status['conditions'] = [{'type': 'Complete', 'status': 'True'}]
      return {'status': desired_status, 'children': []}

    # Compute status based on what we observed, before building desired state.
    # Generate desired state for existing childs.
    desired_childs[child] = new_daemon(job)
    desired_status = copy.deepcopy(children['DaemonSet.apps/v1'].get(child, {}).get('status',{}))

    # If the child is finished the job is finished as well
    if is_job_finished(children['DaemonSet.apps/v1'].get(child, {})):
      desired_status['conditions'] = [{'type': 'Complete', 'status': 'True'}]
      # Cleanup the DeamonJob
      return {'status': desired_status, 'children': []}
    else:
      desired_status['conditions'] = [{'type': 'Complete', 'status': 'False'}]

    return {'status': desired_status, 'children': desired_childs.values()}


  def do_POST(self):
    observed = json.loads(self.rfile.read(int(self.headers.getheader('content-length'))))
    desired = self.sync(observed['parent'], observed['children'])

    self.send_response(200)
    self.send_header('Content-type', 'application/json')
    self.end_headers()
    self.wfile.write(json.dumps(desired))

HTTPServer(('', 80), Controller).serve_forever()
