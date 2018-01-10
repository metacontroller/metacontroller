package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"strings"

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
			vm.TLACode("request", string(body))
			result, err := vm.EvaluateSnippet(filename, hookcode)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, result)
		})
	}

	log.Fatal(http.ListenAndServe(":80", nil))
}
