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

package server

import (
	"fmt"
	"sync"
	"time"

	"metacontroller.io/controller/common"

	"metacontroller.io/controller/decorator"
	"metacontroller.io/options"

	"metacontroller.io/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller.io/client/generated/clientset/internalclientset"
	"metacontroller.io/controller/composite"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type controller interface {
	Start()
	Stop()
}

func Start(configuration options.Configuration) (stop func(), err error) {
	// Create informer factory for metacontroller API objects.
	mcClient, err := mcclientset.NewForConfig(configuration.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("can't create client for api %s: %v", v1alpha1.SchemeGroupVersion, err)
	}

	controllerContext, err := common.NewControllerContext(configuration, mcClient)

	if err != nil {
		return nil, err
	}

	controllers := []controller{
		composite.NewMetacontroller(*controllerContext, mcClient, configuration.Workers),
		decorator.NewMetacontroller(*controllerContext, configuration.Workers),
	}

	controllerContext.Start()

	// Start all controllers.
	for _, c := range controllers {
		c.Start()
	}

	// Return a function that will stop all controllers.
	return func() {
		var wg sync.WaitGroup
		for _, c := range controllers {
			wg.Add(1)
			go func(c controller) {
				defer wg.Done()
				c.Stop()
			}(c)
		}
		wg.Wait()
		time.Sleep(1 * time.Second)
		controllerContext.Stop()
	}, nil
}
