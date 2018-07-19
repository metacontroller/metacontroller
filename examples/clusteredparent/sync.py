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

def new_rolebinding(name):
  rolebinding = {}
  rolebinding['apiVersion'] = 'rbac.authorization.k8s.io/v1'
  rolebinding['kind'] = 'RoleBinding'
  rolebinding['metadata'] = {}
  rolebinding['metadata']['name'] = name
  rolebinding['metadata']['namespace'] = "default"
  rolebinding['subjects'] = [{'kind': 'ServiceAccount', 'name': 'default', 'namespace': 'default'}]
  rolebinding['roleRef'] = {'kind': 'ClusterRole', 'name': name, 'apiGroup': 'rbac.authorization.k8s.io'}
  return rolebinding

class Controller(BaseHTTPRequestHandler):
  def sync(self, clusterrole, children):
    return {'attachments': [new_rolebinding(clusterrole['metadata']['name'])] }


  def do_POST(self):
    observed = json.loads(self.rfile.read(int(self.headers.getheader('content-length'))))
    desired = self.sync(observed['object'], observed['attachments'])

    self.send_response(200)
    self.send_header('Content-type', 'application/json')
    self.end_headers()
    self.wfile.write(json.dumps(desired))

HTTPServer(('', 80), Controller).serve_forever()
