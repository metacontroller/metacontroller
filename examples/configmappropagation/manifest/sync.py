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
from typing import List
import json
import logging

logging.basicConfig(level=logging.DEBUG)
LOGGER = logging.getLogger(__name__)


class Controller(BaseHTTPRequestHandler):
    def sync(self, parent: dict, related: dict) -> List[dict]:
        source_namespace: str = parent['spec']['sourceNamespace']
        source_name: str = parent['spec']['sourceName']
        parent_name = parent['metadata']['name']
        LOGGER.info(f'Processing: {parent_name}')
        if len(related['ConfigMap.v1']) == 0:
            LOGGER.info("Related resource has been deleted, clean-up copies")
            return []
        original_configmap: dict = related['ConfigMap.v1'][f'{source_namespace}/{source_name}']
        target_namespaces: list[str] = parent['spec']['targetNamespaces']
        target_configmaps = [self.new_configmap(
            source_name, namespace, original_configmap['data']) for namespace in target_namespaces]
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

    def customize(self, source_name: str, source_namespace: str) -> List[dict]:
        return [
            {
                'apiVersion': 'v1',
                'resource': 'configmaps',
                'namespace': source_namespace,
                'names': [source_name]
            }
        ]

    def do_POST(self):
        if self.path == '/sync':
            observed: dict = json.loads(self.rfile.read(
                int(self.headers.get('content-length'))))
            parent: dict = observed['parent']
            LOGGER.info("/sync %s", parent['metadata']['name'])
            related: dict = observed['related']
            expected_copies: int = len(parent['spec']['targetNamespaces'])
            actual_copies: int = len(observed['children']['ConfigMap.v1'])
            response: dict = {
                'status': {
                    'expected_copies': expected_copies,
                    'actual_copies': actual_copies
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
