/*
Copyright 2019 Google Inc.

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

package framework

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
)

// ServeWebhook is a helper for quickly creating a webhook server in tests.
func (f *Fixture) ServeWebhook(handler func(request []byte) (response []byte, err error)) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		resp, err := handler(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}))
	f.deferTeardown(func() error {
		srv.Close()
		return nil
	})
	return srv
}
