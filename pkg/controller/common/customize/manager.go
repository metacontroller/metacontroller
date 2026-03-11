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
	"errors"
	"fmt"
	"metacontroller/pkg/controller/common/api"
	commonv2 "metacontroller/pkg/controller/common/api/v2"
	customizecommon "metacontroller/pkg/controller/common/customize/api/common"
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	v2 "metacontroller/pkg/controller/common/customize/api/v2"
	"metacontroller/pkg/hooks"
	"time"

	"k8s.io/apimachinery/pkg/types"
	clientgo_cache "k8s.io/client-go/tools/cache"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/cache"
	"metacontroller/pkg/controller/common"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicinformer "metacontroller/pkg/dynamic/informer"
)

type relatedObjectsSelectionType string

const (
	selectByLabels             relatedObjectsSelectionType = "Labels"
	selectByNamespaceAndNames  relatedObjectsSelectionType = "NamespacesAndNames"
	selectByNamespaceAndLabels relatedObjectsSelectionType = "NamespaceAndLabels"
	selectByNamespaceSelector  relatedObjectsSelectionType = "NamespaceSelector"
	invalid                    relatedObjectsSelectionType = "Invalid"
)

type Manager struct {
	name       string
	controller v1alpha1.CustomizableController

	parentKinds *common.GroupKindMap

	dynClient       *dynamicclientset.Clientset
	dynInformers    *dynamicinformer.SharedInformerFactory
	parentInformers *common.InformerMap

	relatedInformers common.InformerMap
	nsInformer       *dynamicinformer.ResourceInformer
	relatedInformers *common.InformerMap
	customizeCache   *cache.Cache[customizeKey, *v1.CustomizeHookResponse]

	stopCh chan struct{}

	enqueueParent func(interface{})

	customizeHook hooks.Hook

	logger logr.Logger
}

type customizeKey struct {
	uid              types.UID
	parentGeneration int64
}

// newResponseCache returns new, empty response cache.
func newResponseCache() *cache.Cache[customizeKey, *v1.CustomizeHookResponse] {
	return cache.New[customizeKey, *v1.CustomizeHookResponse](20*time.Minute, 10*time.Minute)
}

func NewCustomizeManager(
	name string,
	enqueueParent func(interface{}),
	controller v1alpha1.CustomizableController,
	dynClient *dynamicclientset.Clientset,
	dynInformers *dynamicinformer.SharedInformerFactory,
	parentInformers *common.InformerMap,
	parentKinds *common.GroupKindMap,
	logger logr.Logger,
	controllerType common.ControllerType) (*Manager, error) {
	var hook hooks.Hook
	var err error
	if controller.GetCustomizeHook() != nil {
		hook, err = hooks.NewHook(controller.GetCustomizeHook(), name, controllerType, common.CustomizeHook)
		if err != nil {
			return nil, err
		}
	} else {
		hook = nil
	}
	if parentInformers == nil {
		parentInformers = common.NewInformerMap()
	}
	if parentKinds == nil {
		parentKinds = common.NewGroupKindMap()
	}
	var nsInformer *dynamicinformer.ResourceInformer
	if dynInformers.IsInitialized() {
		nsInformer, err = dynInformers.Resource("v1", "namespaces")
		if err != nil {
			return nil, fmt.Errorf("can't create namespace informer for customize manager: %w", err)
		}
	}

	return &Manager{
		name:             name,
		controller:       controller,
		parentKinds:      parentKinds,
		customizeCache:   newResponseCache(),
		dynClient:        dynClient,
		dynInformers:     dynInformers,
		parentInformers:  parentInformers,
		relatedInformers: common.NewInformerMap(),
		nsInformer:       nsInformer,
		enqueueParent:    enqueueParent,
		customizeHook:    hook,
		logger:           logger,
	}, nil
}

func (rm *Manager) IsEnabled() bool {
	return rm.customizeHook != nil && rm.customizeHook.IsEnabled()
}

func (rm *Manager) Start(stopCh chan struct{}) {
	rm.stopCh = stopCh
}

func (rm *Manager) Stop() {
	rm.relatedInformers.ForEach(func(_ schema.GroupVersionResource, informer *dynamicinformer.ResourceInformer) {
		informer.Informer().RemoveEventHandlers()
		informer.Close()
	})
	if rm.nsInformer != nil {
		rm.nsInformer.Close()
	}
}

func (rm *Manager) getCachedCustomizeHookResponse(parent *unstructured.Unstructured) (*v1.CustomizeHookResponse, bool) {
	return rm.customizeCache.Get(customizeKey{parent.GetUID(), parent.GetGeneration()})
}

func (rm *Manager) getCustomizeHookResponse(parent *unstructured.Unstructured) (*v1.CustomizeHookResponse, error) {
	if !rm.IsEnabled() {
		return nil, nil
	}
	cached, found := rm.getCachedCustomizeHookResponse(parent)
	if found {
		return cached, nil
	}

	hookVersion := rm.customizeHook.GetVersion()

	var requestBuilder customizecommon.WebhookRequestBuilder
	if hookVersion == v1alpha1.HookVersionV2 {
		requestBuilder = v2.NewRequestBuilder()
	} else {
		requestBuilder = v1.NewRequestBuilder()
	}

	request := requestBuilder.
		WithController(rm.controller).
		WithParent(parent).
		Build()

	var v1Response *v1.CustomizeHookResponse
	if hookVersion == v1alpha1.HookVersionV2 {
		var v2Response v2.CustomizeHookResponse
		if err := rm.customizeHook.Call(request, &v2Response); err != nil {
			return nil, err
		}
		v1Response = rm.convertV2ToV1Response(v2Response)
	} else {
		v1Response = &v1.CustomizeHookResponse{}
		if err := rm.customizeHook.Call(request, v1Response); err != nil {
			return nil, err
		}
	}
	v1Response.Version = hookVersion

	rm.customizeCache.Set(customizeKey{parent.GetUID(), parent.GetGeneration()}, v1Response)
	return v1Response, nil
}

func (rm *Manager) convertV2ToV1Response(v2Response v2.CustomizeHookResponse) *v1.CustomizeHookResponse {
	return &v1.CustomizeHookResponse{
		RelatedResourceRules: v2Response.RelatedResourceRules,
	}
}

var ErrRelatedInformerNotSynced = errors.New("related informer not synced yet")

func (rm *Manager) getRelatedClient(apiVersion, resource string) (*dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer, error) {
	client, err := rm.dynClient.Resource(apiVersion, resource)

	if err != nil {
		return nil, nil, err
	}
	groupVersion, _ := schema.ParseGroupVersion(apiVersion)
	gvr := groupVersion.WithResource(resource)

	informer, err := rm.getOrCreateRelatedInformer(apiVersion, resource, gvr)
	if err != nil {
		return nil, nil, err
	}

	if rm.stopCh == nil {
		return nil, nil, fmt.Errorf("customize Manager not started")
	}

	if !informer.Informer().HasSynced() {
		return nil, nil, ErrRelatedInformerNotSynced
	}

	return client, informer, nil
}

func (rm *Manager) getOrCreateRelatedInformer(apiVersion, resource string, gvr schema.GroupVersionResource) (*dynamicinformer.ResourceInformer, error) {
	informer := rm.relatedInformers.Get(gvr)
	if informer != nil {
		return informer, nil
	}

	informer, err := rm.dynInformers.Resource(apiVersion, resource)
	if err != nil {
		return nil, fmt.Errorf("can't create informer for related resource: %w", err)
	}

	actual, loaded := rm.relatedInformers.GetOrCreate(gvr, informer)
	if loaded {
		// If we lost the race, clean up the informer we just created.
		informer.Close()
		return actual, nil
	}

	// We won the race, add event handlers once.
	_, err = actual.Informer().AddEventHandler(clientgo_cache.ResourceEventHandlerFuncs{
		AddFunc:    rm.onRelatedAdd,
		UpdateFunc: rm.onRelatedUpdate,
		DeleteFunc: rm.onRelatedDelete,
	})

	if err != nil {
		// If we fail to add event handlers, we should probably remove the informer from the map
		// and close it so we don't leave a "broken" informer.
		rm.relatedInformers.Delete(gvr)
		actual.Close()
		return nil, fmt.Errorf("can't add event handlers for related resource: %w", err)
	}

	return actual, nil
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
		tombstone, ok := obj.(clientgo_cache.DeletedFinalStateUnknown)
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

	rm.parentInformers.ForEach(func(_ schema.GroupVersionResource, parentInformer *dynamicinformer.ResourceInformer) {
		parents, err := parentInformer.Lister().List(labels.Everything())
		if err != nil {
			return
		}

	MATCHPARENTS:
		for _, parent := range parents {
			customizeHookResponse, err := rm.getCustomizeHookResponse(parent)
			if err != nil || customizeHookResponse == nil {
				// skip for now, the informer relist interval will try again later.
				continue
			}

			for _, relatedRule := range customizeHookResponse.RelatedResourceRules {
				for _, related := range relatedSlice {
					parentGroup, _ := schema.ParseGroupVersion(parent.GetAPIVersion())
					parentResource := rm.parentKinds.Get(schema.GroupKind{Group: parentGroup.Group, Kind: parent.GetKind()})
					if parentResource == nil {
						utilruntime.HandleError(fmt.Errorf("unknown parent %v/%v", parentGroup, parent.GetKind()))
						continue
					}
					relatedRuleClient, _ := rm.dynClient.Resource(relatedRule.APIVersion, relatedRule.Resource)
					if relatedRuleClient == nil {
						utilruntime.HandleError(fmt.Errorf("unknown related rule %v/%v", relatedRule.APIVersion, relatedRule.Resource))
						continue
					}
					matches, err := rm.matchesRelatedRule(customizeHookResponse.Version, parentResource.Namespaced, parent, related, relatedRule, relatedRuleClient.Kind)
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
	})
	return matchingParents
}

func determineSelectionType(relatedRule *v1alpha1.RelatedResourceRule) (relatedObjectsSelectionType, error) {
	hasLabelSelector := relatedRule.LabelSelector != nil
	hasNamespaceSelector := relatedRule.NamespaceSelector != nil
	hasNamespace := len(relatedRule.Namespace) != 0
	hasNames := len(relatedRule.Names) != 0

	// Rule: Explicit list of names cannot be combined with any selector.
	if hasNames && (hasLabelSelector || hasNamespaceSelector) {
		return invalid, fmt.Errorf("related rule cannot have both names and labelSelector/namespaceSelector specified: %#v", relatedRule)
	}

	// Rule: Explicit namespace cannot be combined with namespaceSelector.
	if hasNamespace && hasNamespaceSelector {
		return invalid, fmt.Errorf("related rule cannot have both namespace and namespaceSelector specified: %#v", relatedRule)
	}

	if hasNamespaceSelector {
		return selectByNamespaceSelector, nil
	}
	if hasLabelSelector && hasNamespace {
		return selectByNamespaceAndLabels, nil
	}
	if hasNamespace || hasNames {
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

func (rm *Manager) matchesRelatedRule(hookVersion v1alpha1.HookVersion, parentIsNamespaced bool, parent, related *unstructured.Unstructured, relatedRule *v1alpha1.RelatedResourceRule, relatedRuleKind string) (bool, error) {
	// Ensure that the related resource matches the version and kind of the related rule.
	if related.GetAPIVersion() != relatedRule.APIVersion || related.GetKind() != relatedRuleKind {
		return false, nil
	}

	selectionType, err := determineSelectionType(relatedRule)

	switch selectionType {
	case selectByLabels:
		selector, err := toSelector(relatedRule.LabelSelector)
		if err != nil {
			return false, err
		}
		if !selector.Matches(labels.Set(related.GetLabels())) {
			return false, nil
		}
		if hookVersion == v1alpha1.HookVersionV1 && parentIsNamespaced && related.GetNamespace() != "" && parent.GetNamespace() != related.GetNamespace() {
			return false, nil
		}
		return true, nil
	case selectByNamespaceAndLabels:
		selector, err := toSelector(relatedRule.LabelSelector)
		if err != nil {
			return false, err
		}
		if !selector.Matches(labels.Set(related.GetLabels())) {
			return false, nil
		}
		if related.GetNamespace() != relatedRule.Namespace {
			return false, nil
		}
		return true, nil
	case selectByNamespaceSelector:
		selector, err := toSelector(relatedRule.LabelSelector)
		if err != nil {
			return false, err
		}
		if !selector.Matches(labels.Set(related.GetLabels())) {
			return false, nil
		}
		// If the resource is cluster-scoped, it matches any namespaceSelector
		// (though usually cluster-scoped resources don't have a namespace).
		if related.GetNamespace() == "" {
			return true, nil
		}

		// Check if the related object's namespace matches the namespaceSelector.
		if rm.nsInformer == nil {
			return false, fmt.Errorf("namespace informer is not initialized, cannot use namespaceSelector")
		}
		nsObj, err := rm.nsInformer.Lister().Get(related.GetNamespace())
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil // Namespace not found, definitely no match.
			}
			return false, err
		}
		if nsObj == nil {
			return false, nil
		}

		nsSelector, err := toSelector(relatedRule.NamespaceSelector)
		if err != nil {
			return false, err
		}
		if !nsSelector.Matches(labels.Set(nsObj.GetLabels())) {
			return false, nil
		}
		return true, nil
	case selectByNamespaceAndNames:
		if hookVersion == v1alpha1.HookVersionV1 && parentIsNamespaced {
			parentNamespace := parent.GetNamespace()
			if len(relatedRule.Namespace) != 0 && parentNamespace != relatedRule.Namespace {
				return false, fmt.Errorf("%s: Namespace of parent %s does not match with namespace %s of related rule for %s/%s", parent.GetKind(), parent.GetName(), relatedRule.Namespace, relatedRule.APIVersion, relatedRule.Resource)
			}
			// If related object is namespaced, it must match parent namespace
			if related.GetNamespace() != "" && parentNamespace != related.GetNamespace() {
				return false, nil
			}
		} else if len(relatedRule.Namespace) != 0 && related.GetNamespace() != relatedRule.Namespace {
			// v2 or cluster-scoped parent: objects from any namespace can match, but only if they match the rule
			return false, nil
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

func (rm *Manager) GetRelatedObjects(parent *unstructured.Unstructured) (api.ObjectMap, error) {
	childMap := make(commonv2.UniformObjectMap)
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
			if customizeHookResponse.Version == v1alpha1.HookVersionV1 && parentResource.Namespaced && relatedClient.Namespaced {
				all, err = informer.Lister().Namespace(parentNamespace).List(selector)
			} else {
				all, err = informer.Lister().List(selector)
			}
			if err != nil {
				return nil, fmt.Errorf("can't list %v related objects: %w", relatedClient.Kind, err)
			}
			childMap.InitGroup(relatedClient.GroupVersionKind())
			childMap.InsertAll(parent, all)

		case selectByNamespaceAndLabels:
			selector, err := toSelector(relatedRule.LabelSelector)
			if err != nil {
				return nil, err
			}
			all, err := listObjects(selector, relatedRule.Namespace, informer)
			if err != nil {
				return nil, fmt.Errorf("can't list %v related objects: %w", relatedClient.Kind, err)
			}
			childMap.InitGroup(relatedClient.GroupVersionKind())
			childMap.InsertAll(parent, all)

		case selectByNamespaceSelector:
			if rm.nsInformer == nil {
				return nil, fmt.Errorf("namespace informer is not initialized, cannot use namespaceSelector")
			}
			nsSelector, err := toSelector(relatedRule.NamespaceSelector)
			if err != nil {
				return nil, err
			}

			matchingNamespaces, err := rm.nsInformer.Lister().List(nsSelector)
			if err != nil {
				return nil, fmt.Errorf("can't list namespaces for namespaceSelector: %w", err)
			}

			labelSelector, err := toSelector(relatedRule.LabelSelector)
			if err != nil {
				return nil, err
			}
			if relatedRule.LabelSelector == nil {
				rm.logger.Info("RelatedResourceRule uses namespaceSelector without labelSelector. This will match ALL objects of the specified type in selected namespaces, which may be expensive.", "apiVersion", relatedRule.APIVersion, "resource", relatedRule.Resource)
			}

			childMap.InitGroup(relatedClient.GroupVersionKind())
			for _, ns := range matchingNamespaces {
				all, err := listObjects(labelSelector, ns.GetName(), informer)
				if err != nil {
					return nil, fmt.Errorf("can't list %v related objects in namespace %s: %w", relatedClient.Kind, ns.GetName(), err)
				}
				childMap.InsertAll(parent, all)
			}

		case selectByNamespaceAndNames:
			if customizeHookResponse.Version == v1alpha1.HookVersionV1 && parentResource.Namespaced && relatedClient.Namespaced && len(relatedRule.Namespace) != 0 && parentNamespace != relatedRule.Namespace {
				return nil, fmt.Errorf("requested related object namespace %s differs from parent object namespace %s", relatedRule.Namespace, parentNamespace)
			}
			all, err := listObjects(labels.Everything(), relatedRule.Namespace, informer)
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
