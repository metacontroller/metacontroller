/*
 * Copyright 2026. Metacontroller authors.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package framework

import (
	"context"
	"fmt"
	"metacontroller/pkg/logging"
	"os"
	"os/exec"
	"path/filepath"
)

const installControllerManager = `
Cannot find kube-controller-manager, cannot run integration tests

Please download kube-controller-manager and ensure it is somewhere in the PATH.
See hack/get-kube-binaries.sh

`

// getControllerManagerPath returns a path to a kube-controller-manager executable.
func getControllerManagerPath() (string, error) {
	return exec.LookPath(filepath.Join(binariesPath, "kube-controller-manager"))
}

func createKubeconfig(dir string, server string) (string, error) {
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	content := fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: %s
  name: integration
contexts:
- context:
    cluster: integration
    user: integration
  name: integration
current-context: integration
kind: Config
preferences: {}
users:
- name: integration
  user:
    token: admin-token
`, server)
	if err := os.WriteFile(kubeconfigPath, []byte(content), 0644); err != nil {
		return "", err
	}
	return kubeconfigPath, nil
}

// startControllerManager executes a kube-controller-manager instance.
// The returned function will signal the process and wait for it to exit.
func startControllerManager() (func(), error) {
	cmPath, err := getControllerManagerPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, installControllerManager)
		return nil, fmt.Errorf("could not find kube-controller-manager in PATH: %v", err)
	}

	securePort, err := getAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("could not get a secure port: %v", err)
	}

	kubeconfigPath, err := createKubeconfig(filepath.Dir(SAKeyPath()), ApiserverURL())
	if err != nil {
		return nil, fmt.Errorf("could not create kubeconfig: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(
		ctx,
		cmPath,
		"--kubeconfig", kubeconfigPath,
		"--service-account-private-key-file", SAKeyPath(),
		"--root-ca-file", filepath.Join(filepath.Dir(SAKeyPath()), "apiserver.crt"),
		// We don't need a lot of controllers, just enough to make tests happy.
		"--controllers", "namespace,garbagecollector,serviceaccount",
		"--secure-port", fmt.Sprintf("%d", securePort),
	)

	// Uncomment these to see kube-controller-manager output in test logs.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stop := func() {
		cancel()
		err := cmd.Wait()
		logging.Logger.Info("Kube-controller-manager exit", "status", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run kube-controller-manager: %v", err)
	}
	return stop, nil
}
