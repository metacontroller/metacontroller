#!/usr/bin/env python

# Copyright 2022 Metacontroller authors.
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
import copy
import re
import sys
import uuid
import logging

logging.basicConfig(level=logging.DEBUG)
LOGGER = logging.getLogger(__name__)


def new_event(ew, name, data):
    event = data.copy()
    event['apiVersion'] = 'v1'
    event['kind'] = 'Event'
    metadata = ew.get('metadata', {})
    event['involvedObject'] = {
        'namespace': metadata.get('namespace'),
        'apiVersion': ew.get('apiVersion'),
        'kind': ew.get('kind'),
        'name': metadata.get('name'),
        'resourceVersion': metadata.get('resourceVersion'),
        'uid': metadata.get('uid')
    }

    event['metadata'] = {
        'name': name,
        'labels': {
            'owner': 'ew'
        }
    }
    return event


class Controller(BaseHTTPRequestHandler):
    def sync(self, ew: dict, children: dict) -> dict:
        desired_status = {'conditions': [{'type': 'Completed', 'status': 'True'}], 'active': '1'}
        desired_events = []
        events = ew.get('spec', {}).get('events', [])
        for event in events:
            desired_events.append(new_event(ew, event['name'], {
                'type': event['type'],
                'reason': event['reason']
            }))
        LOGGER.info(desired_events)
        return {'status': desired_status, 'children': desired_events}

    def do_POST(self):
        if self.path == '/sync':
            observed: dict = json.loads(self.rfile.read(int(self.headers.get('content-length'))))
            parent: dict = observed['parent']
            desired = self.sync(parent, observed['children'])
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(desired).encode('utf-8'))
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
