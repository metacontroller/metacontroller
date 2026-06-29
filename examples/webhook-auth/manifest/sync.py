#!/usr/bin/env python3

# Copyright 2026 Metacontroller authors.
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

# Webhook server for the webhook-auth example. Serves HTTPS on port 8443,
# requires mutual TLS (client certificate verified against the CA), and
# enforces a bearer token on every request.

import json
import logging
import ssl
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import List

logging.basicConfig(level=logging.DEBUG)
LOGGER = logging.getLogger(__name__)

CERT_DIR = Path("/certs")
TOKEN_FILE = Path("/token/token")
PORT = 8443


def _expected_token() -> str:
    return TOKEN_FILE.read_text().strip()


class Controller(BaseHTTPRequestHandler):
    def _check_auth(self) -> bool:
        auth = self.headers.get("Authorization", "")
        return auth == "Bearer " + _expected_token()

    def sync(self, parent: dict, related: dict) -> List[dict]:
        source_namespace: str = parent["spec"]["sourceNamespace"]
        source_name: str = parent["spec"]["sourceName"]
        if len(related["Secret.v1"]) == 0:
            LOGGER.info("Related resource has been deleted, clean-up copies")
            return []
        original_secret: dict = related["Secret.v1"][
            f"{source_namespace}/{source_name}"
        ]
        target_namespaces = related["Namespace.v1"]
        target_secrets = []
        for namespace in target_namespaces.values():
            if namespace["metadata"]["name"] != source_namespace:
                target_secrets.append(
                    self.new_secret(
                        source_name,
                        namespace["metadata"]["name"],
                        original_secret["data"],
                    )
                )
        return target_secrets

    def new_secret(self, name: str, namespace: str, data: dict) -> dict:
        return {
            "apiVersion": "v1",
            "kind": "Secret",
            "metadata": {"name": name, "namespace": namespace},
            "data": data,
        }

    def customize(
        self, source_name: str, source_namespace: str, target_label_selector
    ) -> List[dict]:
        return [
            {
                "apiVersion": "v1",
                "resource": "secrets",
                "namespace": source_namespace,
                "names": [source_name],
            },
            {
                "apiVersion": "v1",
                "resource": "namespaces",
                "labelSelector": target_label_selector,
            },
        ]

    def do_POST(self):
        if not self._check_auth():
            LOGGER.warning("Unauthorized request to %s", self.path)
            self.send_response(401)
            self.end_headers()
            return

        body = self.rfile.read(int(self.headers.get("content-length")))

        if self.path == "/sync":
            observed = json.loads(body)
            parent = observed["parent"]
            related = observed["related"]
            LOGGER.info("/sync %s", parent["metadata"]["name"])
            response = {
                "status": {"working": "fine"},
                "children": self.sync(parent, related),
            }
            self.send_response(200)
            self.send_header("Content-type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(response).encode("utf-8"))

        elif self.path == "/customize":
            request = json.loads(body)
            parent = request["parent"]
            LOGGER.info("/customize %s", parent["metadata"]["name"])
            related_resources = {
                "relatedResources": self.customize(
                    parent["spec"]["sourceName"],
                    parent["spec"]["sourceNamespace"],
                    parent["spec"]["targetNamespaceLabelSelector"],
                )
            }
            self.send_response(200)
            self.send_header("Content-type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(related_resources).encode("utf-8"))

        else:
            self.send_response(404)
            self.end_headers()


def main():
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ctx.load_cert_chain(CERT_DIR / "tls.crt", CERT_DIR / "tls.key")
    ctx.load_verify_locations(CERT_DIR / "ca.crt")
    ctx.verify_mode = ssl.CERT_REQUIRED

    server = HTTPServer(("", PORT), Controller)
    server.socket = ctx.wrap_socket(server.socket, server_side=True)
    LOGGER.info("Listening on :%d (HTTPS, mTLS)", PORT)
    server.serve_forever()


if __name__ == "__main__":
    main()
