package server

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller/pkg/client/generated/clientset/internalclientset"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/composite"
	"metacontroller/pkg/controller/decorator"
	"metacontroller/pkg/logging"
	"metacontroller/pkg/options"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func StartCompositeControllerServer(configuration options.Configuration, cc *v1alpha1.CompositeController) error {
	mcClient, err := mcclientset.NewForConfig(configuration.RestConfig)
	if err != nil {
		return err
	}

	runtimeContext, err := common.NewControllerContext(configuration, mcClient)
	if err != nil {
		return err
	}
	runtimeContext.Start()
	runtimeContext.WaitForSync()

	ctrl, err := composite.NewParentController(
		runtimeContext.Resources,
		runtimeContext.DynClient,
		runtimeContext.DynInformers,
		runtimeContext.EventRecorder,
		runtimeContext.McClient,
		runtimeContext.McInformerFactory.Metacontroller().V1alpha1().ControllerRevisions().Lister(),
		cc,
		1,
		logging.Logger.WithName("composite"),
	)
	if err != nil {
		return err
	}
	ctrl.Start()
	ctx := signals.SetupSignalHandler()
	<-ctx.Done()
	ctrl.Stop()

	return nil
}

func StartDecoratorControllerServer(configuration options.Configuration, dc *v1alpha1.DecoratorController) error {
	mcClient, err := mcclientset.NewForConfig(configuration.RestConfig)
	if err != nil {
		return err
	}

	runtimeContext, err := common.NewControllerContext(configuration, mcClient)
	if err != nil {
		return err
	}
	runtimeContext.Start()
	runtimeContext.WaitForSync()

	ctrl, err := decorator.NewDecoratorController(
		runtimeContext.Resources,
		runtimeContext.DynClient,
		runtimeContext.DynInformers,
		runtimeContext.EventRecorder,
		dc,
		1,
		logging.Logger.WithName("decorator"),
	)
	if err != nil {
		return err
	}
	ctrl.Start()
	ctx := signals.SetupSignalHandler()
	<-ctx.Done()
	ctrl.Stop()

	return nil
}
