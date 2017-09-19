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

// This is a generic nodejs server that wraps a function written for
// fission's nodejs environment, making it deployable without fission.

const fs = require('fs');
const path = require('path');
const http = require('http');

let hooks = {};
for (let file of fs.readdirSync('./hooks')) {
  if (file.endsWith('.js')) {
    let name = path.basename(file, '.js');
    console.log('loading hook: /' + name);
    hooks[name] = require('./hooks/' + name);
  }
}

http.createServer((request, response) => {
  let hook = hooks[request.url.split('/')[1]];
  if (!hook) {
    response.writeHead(404, {'Content-Type': 'text/plain'});
    response.end('Not found');
    return;
  }

  // Read the whole request body.
  let body = [];
  request.on('error', (err) => {
    console.error(err);
  }).on('data', (chunk) => {
    body.push(chunk);
  }).on('end', () => {
    body = Buffer.concat(body).toString();

    if (request.headers['content-type'] === 'application/json') {
      body = JSON.parse(body);
    }

    // Emulate part of the fission.io nodejs environment,
    // so we can use the same sync.js file.
    hook({request: {body: body}}).then((result) => {
      response.writeHead(result.status, result.headers);
      let body = result.body;
      if (typeof body !== 'string') {
        body = JSON.stringify(body);
      }
      response.end(body);
    }, (err) => {
      response.writeHead(500, {'Content-Type': 'text/plain'});
      response.end(err.toString());
    });
  });
}).listen(80);
