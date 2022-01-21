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

package customize

import (
	"fmt"
	"metacontroller/pkg/hooks"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicinformer "metacontroller/pkg/dynamic/informer"
)

type relatedObjectsSelectionType string

const (
	selectByLabels            relatedObjectsSelectionType = "Labels"
	selectByNamespaceAndNames relatedObjectsSelectionType = "NamespacesAndNames"
	invalid                   relatedObjectsSelectionType = "Invalid"
)

type Manager struct {
	name       string
	controller CustomizableController

	parentKinds common.GroupKindMap

	dynClient       *dynamicclientset.Clientset
	dynInformers    *dynamicinformer.SharedInformerFactory
	parentInformers common.InformerMap

	relatedInformers common.InformerMap
	customizeCache   *ResponseCache

	stopCh chan struct{}

	enqueueParent func(interface{})

	customizeHook hooks.HookExecutor

	logger logr.Logger
}

func NewCustomizeManager(
	name string,
	enqueueParent func(interface{}),
	controller CustomizableController,
	dynClient *dynamicclientset.Clientset,
	dynInformers *dynamicinformer.SharedInformerFactory,
	parentInformers common.InformerMap,
	parentKinds common.GroupKindMap,
	logger logr.Logger,
	controllerType common.ControllerType) (*Manager, error) {
	var executor hooks.HookExecutor
	var err error
	if controller.GetCustomizeHook() != nil {
		executor, err = hooks.NewHookExecutor(controller.GetCustomizeHook(), name, controllerType, common.CustomizeHook)
		if err != nil {
			return nil, err
		}
	} else {
		executor = nil
	}
	return &Manager{
		name:             name,
		controller:       controller,
		parentKinds:      parentKinds,
		customizeCache:   NewResponseCache(),
		dynClient:        dynClient,
		dynInformers:     dynInformers,
		parentInformers:  parentInformers,
		relatedInformers: make(common.InformerMap),
		enqueueParent:    enqueueParent,
		customizeHook:    executor,
		logger:           logger,
	}, nil
}

func (rm *Manager) IsEnabled() bool {
	return rm.customizeHook != nil
}

func (rm *Manager) Start(stopCh chan struct{}) {
	rm.stopCh = stopCh
}

func (rm *Manager) Stop() {
	for _, informer := range rm.relatedInformers {
		informer.Informer().RemoveEventHandlers()
		informer.Close()
	}
}

func (rm *Manager) getCachedCustomizeHookResponse(parent *unstructured.Unstructured) *CustomizeHookResponse {
	return rm.customizeCache.Get(parent.GetName(), parent.GetGeneration())
}

func (rm *Manager) getCustomizeHookResponse(parent *unstructured.Unstructured) (*CustomizeHookResponse, error) {
	cached := rm.getCachedCustomizeHookResponse(parent)
	if cached != nil {
		return cached, nil
	} else {
		var response CustomizeHookResponse
		request := &CustomizeHookRequest{
			Controller: rm.controller,
			Parent:     parent,
		}
		if err := rm.customizeHook.Execute(request, &response); err != nil {
			return nil, err
		}

		rm.customizeCache.Add(parent.GetName(), parent.GetGeneration(), &response)
		return &response, nil
	}
}

func (rm *Manager) getRelatedClient(apiVersion, resource string) (*dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer, error) {
	client, err := rm.dynClient.Resource(apiVersion, resource)

	if err != nil {
		return nil, nil, err
	}
	groupVersion, _ := schema.ParseGroupVersion(apiVersion)
	informer := rm.relatedInformers.Get(groupVersion.WithResource(resource))
	if informer == nil {
		informer, err = rm.dynInformers.Resource(apiVersion, resource)

		if err != nil {
			return nil, nil, fmt.Errorf("can't create informer for related resource: %w", err)
		}

		informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    rm.onRelatedAdd,
			UpdateFunc: rm.onRelatedUpdate,
			DeleteFunc: rm.onRelatedDelete,
		})

		if !cache.WaitForNamedCacheSync(rm.name, rm.stopCh, informer.Informer().HasSynced) {
			rm.logger.Info("related Manager - cache sync never finished", "name", rm.name)
		}

		groupVersion, _ := schema.ParseGroupVersion(apiVersion)
		rm.relatedInformers.Set(groupVersion.WithResource(resource), informer)
	}

	return client, informer, nil
}

func (rm *Manager) onRelatedAdd(obj interface{}) {
	related := obj.(*unstructured.Unstructured)

	if related.GetDeletionTimestamp() != nil {
		rm.onRelatedDelete(related)
		return
	}

	rm.notifyRelatedParents(related)
}

func (rm *Manager) onRelatedUpdate(old, cur interface{}) {
	oldRelated := old.(*unstructured.Unstructured)
	curRelated := cur.(*unstructured.Unstructured)

	// We don't care about no-op updates. See onChildUpdate for the reason.
	if oldRelated.GetResourceVersion() == curRelated.GetResourceVersion() {
		return
	}

	// We want to notify parents that are interested in the new state or were interested
	// in the old state.
	rm.notifyRelatedParents(oldRelated, curRelated)
}

func (rm *Manager) onRelatedDelete(obj interface{}) {
	related, ok := obj.(*unstructured.Unstructured)

	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		related, ok = tombstone.Obj.(*unstructured.Unstructured)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not *unstructured.Unstructured %#v", obj))
			return
		}
	}

	rm.notifyRelatedParents(related)
}

func (rm *Manager) notifyRelatedParents(related ...*unstructured.Unstructured) {
	parents := rm.findRelatedParents(related...)
	if len(parents) == 0 {
		return
	}
	for _, parent := range parents {
		rm.enqueueParent(parent)
	}
}

func (rm *Manager) findRelatedParents(relatedSlice ...*unstructured.Unstructured) []*unstructured.Unstructured {
	var matchingParents []*unstructured.Unstructured

	for _, parentInformer := range rm.parentInformers {
		parents, err := parentInformer.Lister().List(labels.Everything())
		if err != nil {
			return nil
		}

	MATCHPARENTS:
		for _, parent := range parents {
			customizeHookResponse := rm.getCachedCustomizeHookResponse(parent)

			if customizeHookResponse == nil {
				continue
			}

			for _, relatedRule := range customizeHookResponse.RelatedResourceRules {
				for _, related := range relatedSlice {
					matches, err := rm.matchesRelatedRule(parent, related, relatedRule)
					if err != nil {
						utilruntime.HandleError(err)
						continue
					}
					if matches {
						matchingParents = append(matchingParents, parent)
						continue MATCHPARENTS
					}
				}
			}
		}
	}
	return matchingParents
}

func determineSelectionType(relatedRule *v1alpha1.RelatedResourceRule) (relatedObjectsSelectionType, error) {
	hasLabelSelector := relatedRule.LabelSelector != nil
	hasNamespaceOrNames := len(relatedRule.Namespace) != 0 || len(relatedRule.Names) != 0
	if hasLabelSelector && hasNamespaceOrNames {
		return invalid, fmt.Errorf("related rule cannot have both labelSelector and Namespace/Names specifcied : %#v", relatedRule)
	}
	if hasNamespaceOrNames {
		return selectByNamespaceAndNames, nil
	}
	return selectByLabels, nil
}

func stringInArray(toMatch string, array []string) bool {
	for _, element := range array {
		if toMatch == element {
			return true
		}
	}
	return false
}

func toSelector(labelSelector *metav1.LabelSelector) (labels.Selector, error) {
	if labelSelector == nil {
		return labels.Everything(), nil
	} else {
		return metav1.LabelSelectorAsSelector(labelSelector)
	}
}

func (rm *Manager) matchesRelatedRule(parent, related *unstructured.Unstructured, relatedRule *v1alpha1.RelatedResourceRule) (bool, error) {
	parentGroup, _ := schema.ParseGroupVersion(parent.GetAPIVersion())
	parentResource := rm.parentKinds.Get(schema.GroupKind{Group: parentGroup.Group, Kind: parent.GetKind()})
	if parentResource == nil {
		return false, fmt.Errorf("unknown parent %v/%v", parentGroup, parent.GetKind())
	}

	selectionType, err := determineSelectionType(relatedRule)

	switch selectionType {
	case selectByLabels:
		selector, err := toSelector(relatedRule.LabelSelector)
		if err != nil {
			return false, err
		}
		relatedLabels := related.GetLabels()
		return selector.Matches(labels.Set(relatedLabels)), nil
	case selectByNamespaceAndNames:
		if parentResource.Namespaced {
			parentNamespace := parent.GetNamespace()
			relatedNamespace := related.GetNamespace()
			if len(relatedRule.Namespace) == 0 && relatedNamespace == parentNamespace {
				return true, nil
			} else if relatedRule.Namespace == "*" || relatedRule.Namespace == relatedNamespace {
				return true, nil
			} else {
				return false, fmt.Errorf("could not create namespace for {relatedRule.namespace: %s, parentNamespace : %s, relatedNamespace : %s } ", relatedRule.Namespace, parentNamespace, relatedNamespace)
			}
		}
		if len(relatedRule.Names) != 0 {
			relatedName := related.GetName()
			return stringInArray(relatedName, relatedRule.Names), nil
		}
		return true, nil
	case invalid:
		return false, err
	}
	return false, fmt.Errorf("should not reach here")
}

func listObjects(selector labels.Selector, namespace string, informer *dynamicinformer.ResourceInformer) ([]*unstructured.Unstructured, error) {
	if len(namespace) != 0 {
		return informer.Lister().Namespace(namespace).List(selector)
	}
	return informer.Lister().List(selector)
}

func (rm *Manager) GetRelatedObjects(parent *unstructured.Unstructured) (common.RelativeObjectMap, error) {
	childMap := make(common.RelativeObjectMap)
	if !rm.IsEnabled() {
		return childMap, nil
	}
	parentGroup, _ := schema.ParseGroupVersion(parent.GetAPIVersion())
	parentResource := rm.parentKinds.Get(schema.GroupKind{Group: parentGroup.Group, Kind: parent.GetKind()})
	if parentResource == nil {
		return nil, fmt.Errorf("unknown parent %v/%v", parentGroup, parent.GetKind())
	}

	parentNamespace := parent.GetNamespace()

	customizeHookResponse, err := rm.getCustomizeHookResponse(parent)

	if err != nil {
		return nil, err
	}

	for _, relatedRule := range customizeHookResponse.RelatedResourceRules {
		relatedClient, informer, err := rm.getRelatedClient(relatedRule.APIVersion, relatedRule.Resource)
		if err != nil {
			return nil, err
		}

		selectionType, err := determineSelectionType(relatedRule)

		switch selectionType {
		case selectByLabels:
			selector, err := toSelector(relatedRule.LabelSelector)
			if err != nil {
				return nil, err
			}
			var all []*unstructured.Unstructured
			if parentResource.Namespaced && relatedRule.Namespace != "*" {
				all, err = informer.Lister().Namespace(parentNamespace).List(selector)
			} else {
				all, err = informer.Lister().List(selector)
			}
			if err != nil {
				return nil, fmt.Errorf("can't list %v related objects: %w", relatedClient.Kind, err)
			}
			childMap.InitGroup(relatedClient.GroupVersionKind())
			childMap.InsertAll(parent, all)

		case selectByNamespaceAndNames:
			if parentResource.Namespaced && len(relatedRule.Namespace) != 0 && parentNamespace != relatedRule.Namespace && relatedRule.Namespace != "*" {
				return nil, fmt.Errorf("requested related object namespace %s differs from parent object namespace %s", relatedRule.Namespace, parentNamespace)
			}
			var all []*unstructured.Unstructured
			if relatedRule.Namespace == "*" {
				all, err = informer.Lister().List(labels.Everything())
			} else {
				all, err = listObjects(labels.Everything(), relatedRule.Namespace, informer)
			}
			if err != nil {
				return nil, fmt.Errorf("can't list %v related objects: %w", relatedClient.Kind, err)
			}
			childMap.InitGroup(relatedClient.GroupVersionKind())
			if len(relatedRule.Names) == 0 {
				childMap.InsertAll(parent, all)
			} else {
				for _, obj := range all {
					if stringInArray(obj.GetName(), relatedRule.Names) {
						childMap.Insert(parent, obj)
					}
				}
			}
		case invalid:
			return nil, err
		}
	}
	return childMap, err
}
