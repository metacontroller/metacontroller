package options

import (
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

type Configuration struct {
	RestConfig        *rest.Config
	DiscoveryInterval time.Duration
	InformerRelist    time.Duration
	Workers           int
	CorrelatorOptions record.CorrelatorOptions
	MetricsEndpoint   string
}
