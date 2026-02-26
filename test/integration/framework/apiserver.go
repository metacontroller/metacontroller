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

/*
This file replaces the mechanism for starting kube-apiserver used in
k8s.io/kubernetes integration tests. In k8s.io/kubernetes, the apiserver is
one of the components being tested, so it makes sense that there we build it
from scratch and link it into the test binary. However, here we treat the
apiserver as an external component just like etcd. This avoids having to vendor
and build all of Kubernetes into our test binary.
*/

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"metacontroller/pkg/logging"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"k8s.io/client-go/rest"
)
 
 var apiserverURL = ""
 var saKeyPath = ""
 
 const installApiserver = `
 Cannot find kube-apiserver, cannot run integration tests
 
 See hack/get-kube-binaries.sh
 
 `
 
 // SAKeyPath returns the path to the service account key.
 func SAKeyPath() string {
	return saKeyPath
 }
 
 // getApiserverPath returns a path to a kube-apiserver executable.
 func getApiserverPath() (string, error) {
	return exec.LookPath(filepath.Join(binariesPath, "kube-apiserver"))
 }
 
 func generateSAKey(keyPath string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
 
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
 
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return err
	}
 
	return nil
 }
 
 func createTokenFile(tokenPath string) error {
	content := "admin-token,admin,admin,system:masters\n"
	return os.WriteFile(tokenPath, []byte(content), 0644)
 }
 
 // startApiserver executes a kube-apiserver instance.
 // The returned function will signal the process and wait for it to exit.
 func startApiserver() (func(), error) {
	apiserverPath, err := getApiserverPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, installApiserver)
		return nil, fmt.Errorf("could not find kube-apiserver in PATH: %v", err)
	}
	apiserverSecurePort, err := getAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("could not get a secure port: %v", err)
	}
	apiserverURL = fmt.Sprintf("https://127.0.0.1:%d", apiserverSecurePort)
	logging.Logger.Info("Starting kube-apiserver", "url", apiserverURL)
 
	apiserverDataDir, err := os.MkdirTemp(os.TempDir(), "integration_test_apiserver_data")
	if err != nil {
		return nil, fmt.Errorf("unable to make temp kube-apiserver data dir: %v", err)
	}
	logging.Logger.Info("Storing kube-apiserver data", "data_directory", apiserverDataDir)

	saKeyPath = filepath.Join(apiserverDataDir, "sa.key")
	if err := generateSAKey(saKeyPath); err != nil {
		return nil, fmt.Errorf("unable to generate service account key: %v", err)
	}

	tokenPath := filepath.Join(apiserverDataDir, "tokens.csv")
	if err := createTokenFile(tokenPath); err != nil {
		return nil, fmt.Errorf("unable to create token file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(
		ctx,
		apiserverPath,
		"--cert-dir", apiserverDataDir,
		"--secure-port", strconv.Itoa(apiserverSecurePort),
		"--etcd-servers", etcdURL,
		"--external-hostname", "localhost",
		"--service-account-issuer", "https://kubernetes.default.svc.cluster.local",
		"--service-account-key-file", saKeyPath,
		"--service-account-signing-key-file", saKeyPath,
		"--token-auth-file", tokenPath,
		"--authorization-mode", "AlwaysAllow",
	)

	// Uncomment these to see kube-apiserver output in test logs.
	// For Metacontroller tests, we generally don't expect problems at this level.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stop := func() {
		cancel()
		err := cmd.Wait()
		logging.Logger.Info("Kube-apiserver exit", "status", err)
		err = os.RemoveAll(apiserverDataDir)
		if err != nil {
			logging.Logger.Error(err, "Error during kube-apiserver cleanup")
		}
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run kube-apiserver: %v", err)
	}
	return stop, nil
}

// ApiserverURL returns the URL of the kube-apiserver instance started by TestMain.
func ApiserverURL() string {
	return apiserverURL
}

// ApiserverConfig returns a rest.Config to connect to the test instance.
func ApiserverConfig() *rest.Config {
	return &rest.Config{
		Host: ApiserverURL(),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
		BearerToken: "admin-token",
	}
}
