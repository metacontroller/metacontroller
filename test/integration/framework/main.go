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
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"metacontroller/pkg/options"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	"metacontroller/pkg/server"
)

var resourceMap *dynamicdiscovery.ResourceMap

const installKubectl = `
Cannot find kubectl, cannot run integration tests

Please download kubectl and ensure it is somewhere in the PATH.
See hack/get-kube-binaries.sh

`

// manifestDir is the path from the integration test binary working dir to the
// directory containing manifests to install Metacontroller.
const manifestDir = "../../../manifests/production"

// getKubectlPath returns a path to a kube-apiserver executable.
func getKubectlPath() (string, error) {
	return exec.LookPath("kubectl")
}

// TestMain starts etcd, kube-apiserver, and metacontroller before running tests.
func TestMain(tests func() int) {
	klog.InitFlags(nil)
	flag.Parse()
	if err := testMain(tests); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func testMain(tests func() int) error {
	if _, err := getKubectlPath(); err != nil {
		return errors.New(installKubectl)
	}

	stopEtcd, err := startEtcd()
	if err != nil {
		return fmt.Errorf("cannot run integration tests: unable to start etcd: %v", err)
	}
	defer stopEtcd()

	stopApiserver, err := startApiserver()
	if err != nil {
		return fmt.Errorf("cannot run integration tests: unable to start kube-apiserver: %v", err)
	}
	defer stopApiserver()

	klog.Info("Waiting for kube-apiserver to be ready...")
	start := time.Now()
	for {
		time.Sleep(time.Second)
		kubectlErr := execKubectl("get", "namespaces")
		if kubectlErr == nil {
			break
		}
		klog.ErrorS(kubectlErr, "Kubectl error")
		if time.Since(start) > time.Minute {
			return fmt.Errorf("timed out waiting for kube-apiserver to be ready: %v", kubectlErr)
		}
	}
	klog.Info("Kube-apiserver started")
	// Create Metacontroller Namespace.
	if err := execKubectl("apply", "-f", path.Join(manifestDir, "metacontroller-namespace.yaml")); err != nil {
		return fmt.Errorf("cannot install metacontroller namespace: %v", err)
	}

	// Install Metacontroller RBAC.
	if err := execKubectl("apply", "-f", path.Join(manifestDir, "metacontroller-rbac.yaml")); err != nil {
		return fmt.Errorf("cannot install metacontroller RBAC: %v", err)
	}

	// Install Metacontroller CRDs.
	if err := execKubectl("apply", "-f", path.Join(manifestDir, "metacontroller-crds-v1.yaml")); err != nil {
		return fmt.Errorf("cannot install metacontroller CRDs: %v", err)
	}

	// Wait for CRDs to be created
	if err := execKubectl("wait", "--for=condition=Established", "crd", "compositecontrollers.metacontroller.k8s.io"); err != nil {
		return fmt.Errorf("cannot install metacontroller CRDs: %v", err)
	}
	if err := execKubectl("wait", "--for=condition=Established", "crd", "decoratorcontrollers.metacontroller.k8s.io"); err != nil {
		return fmt.Errorf("cannot install metacontroller CRDs: %v", err)
	}

	// In this integration test environment, there are no Nodes, so the
	// metacontroller StatefulSet will not actually run anything.
	// Instead, we start the Metacontroller server locally inside the test binary,
	// since that's part of the code under test.
	configuration := options.Configuration{
		RestConfig:        ApiserverConfig(),
		DiscoveryInterval: 500 * time.Millisecond,
		InformerRelist:    30 * time.Minute,
		Workers:           5,
		CorrelatorOptions: record.CorrelatorOptions{},
	}
	mgr, stopServer, err := server.New(configuration)
	if err != nil {
		return fmt.Errorf("cannot create a metacontroller server: %v", err)
	}
	mgrStopChan := signals.SetupSignalHandler()
	defer stopServer()
	go func() {
		if err := mgr.Start(mgrStopChan); err != nil {
			klog.ErrorS(err, "Terminating")
			os.Exit(1)
		}
	}()

	// Periodically refresh discovery to pick up newly-installed resources.
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(ApiserverConfig())
	resourceMap = dynamicdiscovery.NewResourceMap(discoveryClient)
	// We don't care about stopping this cleanly since it has no external effects.
	resourceMap.Start(500 * time.Millisecond)

	// Now actually run the tests.
	if exitCode := tests(); exitCode != 0 {
		return fmt.Errorf("one or more tests failed with exit code: %v", exitCode)
	}
	return nil
}

func execKubectl(args ...string) error {
	execPath, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("cannot exec kubectl: %v", err)
	}
	cmdline := append([]string{"--server", ApiserverURL()}, args...)
	klog.InfoS("Executing command", "command", execPath, "arguments", cmdline)
	cmd := exec.Command(execPath, cmdline...)
	return cmd.Run()
}
