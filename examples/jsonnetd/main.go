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

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	jsonnet "github.com/google/go-jsonnet"
)

func main() {
	// Read all Jsonnet files in the working dir.
	files, err := ioutil.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		filename := file.Name()
		if !strings.HasSuffix(filename, ".jsonnet") {
			continue
		}

		hookname := strings.TrimSuffix(filename, ".jsonnet")
		filedata, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Fatalf("can't read %q: %v", filename, err)
		}
		hookcode := string(filedata)

		// Serve the Jsonnet file as a webhook.
		http.HandleFunc("/"+hookname, func(w http.ResponseWriter, r *http.Request) {
			// Read POST body as jsonnet input.
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Can't read body", http.StatusInternalServerError)
				return
			}

			// Evaluate Jsonnet hook, passing request body as a top-level argument.
			vm := jsonnet.MakeVM()
			for _, ext := range extensions {
				vm.NativeFunction(ext)
			}
			vm.TLACode("request", string(body))
			result, err := vm.EvaluateSnippet(filename, hookcode)
			if err != nil {
				log.Printf("/%s request: %s", hookname, body)
				log.Printf("/%s error: %s", hookname, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, result)
		})
	}

	server := &http.Server{Addr: ":8080"}
	go func() {
		log.Fatal(server.ListenAndServe())
	}()

	// Shutdown on SIGTERM.
	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigchan
	log.Printf("Received %v signal. Shutting down...", sig)
	server.Shutdown(context.Background())
}
