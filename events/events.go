package events

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

const (
	ReasonStarted   string = "Started"
	ReasonStarting  string = "Starting"
	ReasonStopped   string = "Stopped"
	ReasonStopping  string = "Stopping"
	ReasonSyncError string = "SyncError"
)

func NewBroadcaster(config *rest.Config, options record.CorrelatorOptions) (record.EventBroadcaster, error) {
	broadcaster := record.NewBroadcasterWithCorrelatorOptions(options)

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events(metav1.NamespaceAll)})
	return broadcaster, nil
}
