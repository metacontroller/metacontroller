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
import json
import logging
import typing

logging.basicConfig(level=logging.DEBUG)
LOGGER = logging.getLogger(__name__)


class Controller(BaseHTTPRequestHandler):
    def sync(self, parent: dict, related: dict) -> dict:
        sourceNamespace: str = parent['spec']['sourceNamespace']
        sourceName: str = parent['spec']['sourceName']
        original_configmap: dict = related['ConfigMap.v1'][f'{sourceNamespace}/{sourceName}']
        targetNamespaces = related['Namespace.v1']
        target_configmaps = []
        for namespace in targetNamespaces.values():
            if namespace['metadata']['name'] != sourceNamespace:
                target_configmaps.append(self.new_configmap(
                    sourceName, namespace['metadata']['name'], original_configmap['data']))
        return target_configmaps

    def new_configmap(self, name: str, namespace: str, data: dict) -> dict:
        return {
            'apiVersion': 'v1',
            'kind': 'ConfigMap',
            'metadata': {
                'name': name,
                'namespace': namespace
            },
            'data': data
        }

    def customize(self, sourceName: str, sourceNamespace: str) -> dict:
        return [
            {
                'apiVersion': 'v1',
                'resource': 'configmaps',
                'labelSelector': {},
                'namespace': sourceNamespace,
                'names': [sourceName]
            }, {
                'apiVersion': 'v1',
                'resource': 'namespaces',
                'labelSelector': {}
            }
        ]

    def do_POST(self):
        if self.path == '/sync':
            observed: dict = json.loads(self.rfile.read(
                int(self.headers.get('content-length'))))
            parent: dict = observed['parent']
            related: dict = observed['related']
            LOGGER.info("/sync %s", parent['metadata']['name'])
            response: dict = {
                'status': {
                    'working': 'fine'
                },
                'children': self.sync(parent, related)
            }
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(response).encode('utf-8'))
        elif self.path == '/customize':
            request: dict = json.loads(self.rfile.read(
                int(self.headers.get('content-length'))))
            parent: dict = request['parent']
            LOGGER.info("/customize %s", parent['metadata']['name'])
            related_resources: dict = {
                'relatedResources': self.customize(
                    parent['spec']['sourceName'],
                    parent['spec']['sourceNamespace']
                )
            }
            LOGGER.info("Related resources: \n %s", related_resources)
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(related_resources).encode('utf-8'))
        else:
            self.send_response(404)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            error_msg: dict = {
                'error': '404',
                'endpoint': self.path
            }
            self.wfile.write(json.dumps(error_msg).encode('utf-8'))


HTTPServer(('', 80), Controller).serve_forever()
