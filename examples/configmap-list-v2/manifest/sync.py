#!/usr/bin/env python

from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import logging

logging.basicConfig(level=logging.DEBUG)
LOGGER = logging.getLogger(__name__)

class Controller(BaseHTTPRequestHandler):
    def sync(self, parent, children, related):
        parent_name = parent['metadata']['name']
        parent_ns = parent['metadata']['namespace']
        LOGGER.info(f"Syncing parent {parent_ns}/{parent_name}")

        # In v2, keys in related are always "namespace/name" for namespaced resources
        configmaps = related.get('ConfigMap.v1', {})
        cm_list = sorted(configmaps.keys())
        
        # Return a child ConfigMap in the same namespace as the parent
        child_cm = {
            'apiVersion': 'v1',
            'kind': 'ConfigMap',
            'metadata': {
                'name': parent_name + "-list",
                'namespace': parent_ns
            },
            'data': {
                'configmaps': "\n".join(cm_list)
            }
        }

        return {
            'status': {
                'count': len(cm_list)
            },
            'children': [child_cm]
        }

    def customize(self, parent):
        source_ns = parent['spec']['sourceNamespace']
        source_labels = parent['spec']['sourceLabels']
        
        # Use RelatedResourceRule to request ConfigMaps from another namespace
        # This is a v2-specific capability for namespaced parents
        return {
            'relatedResources': [
                {
                    'apiVersion': 'v1',
                    'resource': 'configmaps',
                    'labelSelector': {
                        'matchLabels': source_labels
                    }
                }
            ]
        }

    def do_POST(self):
        content_length = int(self.headers['Content-Length'])
        request_body = json.loads(self.rfile.read(content_length))

        if self.path == '/sync':
            parent = request_body['parent']
            children = request_body['children']
            related = request_body['related']
            response = self.sync(parent, children, related)
        elif self.path == '/customize':
            parent = request_body['parent']
            response = self.customize(parent)
        else:
            self.send_response(404)
            return

        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(response).encode('utf-8'))

if __name__ == '__main__':
    LOGGER.info("Starting controller on port 80...")
    HTTPServer(('', 80), Controller).serve_forever()
