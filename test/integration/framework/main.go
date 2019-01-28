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
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/klog"

	dynamicdiscovery "metacontroller.app/dynamic/discovery"
	"metacontroller.app/server"
)

var resourceMap *dynamicdiscovery.ResourceMap

const installKubectl = `
Cannot find kubectl, cannot run integration tests

Please download kubectl and ensure it is somewhere in the PATH.
See hack/get-kube-binaries.sh

`

// manifestDir is the path from the integration test binary working dir to the
// directory containing manifests to install Metacontroller.
const manifestDir = "../../../manifests"

// getKubectlPath returns a path to a kube-apiserver executable.
func getKubectlPath() (string, error) {
	return exec.LookPath("kubectl")
}

// TestMain starts etcd, kube-apiserver, and metacontroller before running tests.
func TestMain(tests func() int) {
	result := 1
	defer func() {
		os.Exit(result)
	}()

	if _, err := getKubectlPath(); err != nil {
		klog.Fatal(installKubectl)
	}

	stopEtcd, err := startEtcd()
	if err != nil {
		klog.Fatalf("cannot run integration tests: unable to start etcd: %v", err)
	}
	defer stopEtcd()

	stopApiserver, err := startApiserver()
	if err != nil {
		klog.Fatalf("cannot run integration tests: unable to start kube-apiserver: %v", err)
	}
	defer stopApiserver()

	klog.Info("Waiting for kube-apiserver to be ready...")
	start := time.Now()
	for {
		if err := execKubectl("version"); err == nil {
			break
		}
		if time.Since(start) > defaultWaitTimeout {
			klog.Fatalf("timed out waiting for kube-apiserver to be ready: %v", err)
		}
		time.Sleep(time.Second)
	}

	// Create Metacontroller Namespace.
	if err := execKubectl("apply", "-f", path.Join(manifestDir, "metacontroller-namespace.yaml")); err != nil {
		klog.Fatalf("can't install metacontroller namespace: %v", err)
	}

	// Install Metacontroller RBAC.
	if err := execKubectl("apply", "-f", path.Join(manifestDir, "metacontroller-rbac.yaml")); err != nil {
		klog.Fatalf("can't install metacontroller RBAC: %v", err)
	}

	// Install Metacontroller CRDs.
	if err := execKubectl("apply", "-f", path.Join(manifestDir, "metacontroller.yaml")); err != nil {
		klog.Fatalf("can't install metacontroller CRDs: %v", err)
	}

	// In this integration test environment, there are no Nodes, so the
	// metacontroller StatefulSet will not actually run anything.
	// Instead, we start the Metacontroller server locally inside the test binary,
	// since that's part of the code under test.
	stopServer, err := server.Start(ApiserverConfig(), 500*time.Millisecond, 30*time.Minute)
	if err != nil {
		klog.Fatalf("can't start metacontroller server: %v", err)
	}
	defer stopServer()

	// Periodically refresh discovery to pick up newly-installed resources.
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(ApiserverConfig())
	resourceMap = dynamicdiscovery.NewResourceMap(discoveryClient)
	// We don't care about stopping this cleanly since it has no external effects.
	resourceMap.Start(500 * time.Millisecond)

	result = tests()
}

func execKubectl(args ...string) error {
	execPath, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("can't exec kubectl: %v", err)
	}
	cmdline := append([]string{"--server", ApiserverURL()}, args...)
	cmd := exec.Command(execPath, cmdline...)
	return cmd.Run()
}
