## Runtime Profiling with pprof

### Brief Overview
[pprof](https://github.com/google/pprof) is a tool for visualization and analysis of profiling data.

pprof reads a collection of profiling samples in profile.proto format and generates reports to visualize and help analyze the data. It can generate both text and graphical reports (through the use of the dot visualization package).

Importing [net/http/pprof](https://pkg.go.dev/net/http/pprof) creates an HTTP interface exposing [runtime/pprof](https://pkg.go.dev/runtime/pprof).
Please see [Go Diagnostics](https://golang.org/doc/diagnostics) for additional information on how to profile and run diagnostics in go.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

### Enable pprof
Once enabled, the endpoint `/debug/pprof` will be served on the address provided.
- Set the metacontroller command-line argument pprof-address to the desired address. (e.g., `:6060`)
  ```
  args:
  - --pprof-address=:6060
  ```

### Disable pprof
Once disabled, the endpoint `/debug/pprof` will be unavailable.
- Set the metacontroller command-line argument pprof-address to 0.
  ```
  args:
  - --pprof-address=0
  ```

### Profiling metacontroller
- Port-forward metacontroller `kubectl -n metacontroller port-forward statefulset/metacontroller 6060`
- Open http://localhost:6060/debug/pprof/ in a web browser
- Select your desired profile.

#### Example for heap profiling
- Navigate to http://localhost:6060/debug/pprof/heap?debug=1
  - Heap profiling data will be displayed in the web browser
- Navigate to http://localhost:6060/debug/pprof/heap?debug=0
  - A heap snapshot will be downloaded
- Run the following cli command on the file
  - `go tool pprof heap` where heap is assumed to be the name of the file
  - Execute pprof commands, for a full list of commands visit https://github.com/google/pprof
    - `top 20`
      - Displays the top 20 nodes consuming heap memory
    - `web`
      - Opens a visual flowchart of function calls and associated memory usage
- Additionally, you can profile directly without the intermediate step of downloading a file
  - `go tool pprof -top 20 http://localhost:6060/debug/pprof/heap`