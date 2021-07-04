package main

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/logging"
	"metacontroller/pkg/options"
	"metacontroller/pkg/server"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	logging.InitLogging(&zap.Options{})
	klog.InitFlags(nil)
	configuration := options.NewConfiguration(
		options.WithRestConfig(config.GetConfigOrDie()),
	)
	webhookUrl := "http://cluster-parent-controller.metacontroller/sync"
	dc := v1alpha1.DecoratorController{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CompositeController",
			APIVersion: "metacontroller.k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "bluegreen-controller",
		},
		Spec: v1alpha1.DecoratorControllerSpec{
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{
					ResourceRule: v1alpha1.ResourceRule{
						APIVersion: "rbac.authorization.k8s.io/v1",
						Resource:   "clusterroles",
					},
					AnnotationSelector: &v1alpha1.AnnotationSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "default-service-account-binding",
								Operator: metav1.LabelSelectorOpExists,
							},
						},
					},
				},
			},
			Attachments: []v1alpha1.DecoratorControllerAttachmentRule{
				{
					ResourceRule: v1alpha1.ResourceRule{
						APIVersion: "rbac.authorization.k8s.io/v1",
						Resource:   "rolebindings",
					},
				},
			},
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &webhookUrl,
					},
				},
			},
		},
	}
	if err := server.StartDecoratorControllerServer(configuration, &dc); err != nil {
		klog.ErrorS(err, "Could not start decorator controller")
	}
}
