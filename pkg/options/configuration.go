/*
Copyright 2021 Metacontroller authors.

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

package options

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/leaderelection"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

type Configuration struct {
	RestConfig            *rest.Config
	DiscoveryInterval     time.Duration
	InformerRelist        time.Duration
	Workers               int
	CorrelatorOptions     record.CorrelatorOptions
	MetricsEndpoint       string
	LeaderElectionOptions leaderelection.Options
	Api                   bool
	ApiPort               string
	ApiTriggerSync        bool
}
