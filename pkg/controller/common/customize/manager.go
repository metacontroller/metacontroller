package customize

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

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
	name           string
	metacontroller CustomizableController

	parentKinds common.GroupKindMap

	dynClient       *dynamicclientset.Clientset
	dynInformers    *dynamicinformer.SharedInformerFactory
	parentInformers common.InformerMap

	relatedInformers common.InformerMap
	customizeCache   *ResponseCache

	stopCh chan struct{}

	enqueueParent func(interface{})
}

func NewCustomizeManager(
	name string,
	enqueueParent func(interface{}),
	metacontroller CustomizableController,
	dynClient *dynamicclientset.Clientset,
	dynInformers *dynamicinformer.SharedInformerFactory,
	parentInformers common.InformerMap,
	parentKinds common.GroupKindMap,
) Manager {
	return Manager{
		name:             name,
		metacontroller:   metacontroller,
		parentKinds:      parentKinds,
		customizeCache:   NewResponseCache(),
		dynClient:        dynClient,
		dynInformers:     dynInformers,
		parentInformers:  parentInformers,
		relatedInformers: make(common.InformerMap),
		enqueueParent:    enqueueParent,
	}
}

func (rm *Manager) Start(stopCh chan struct{}) {
	rm.stopCh = stopCh
}

func (rm *Manager) GetCachedCustomizeHookResponse(parent *unstructured.Unstructured) *CustomizeHookResponse {
	return rm.customizeCache.Get(parent.GetName(), parent.GetGeneration())
}

func (rm *Manager) GetCustomizeHookResponse(parent *unstructured.Unstructured) (*CustomizeHookResponse, error) {
	cached := rm.GetCachedCustomizeHookResponse(parent)
	if cached != nil {
		return cached, nil
	} else {
		response, err := CallCustomizeHook(rm.metacontroller, &CustomizeHookRequest{
			Controller: rm.metacontroller,
			Parent:     parent,
		})
		if err != nil {
			return nil, err
		}

		rm.customizeCache.Add(parent.GetName(), parent.GetGeneration(), response)
		return response, nil
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
			klog.InfoS("related Manager - cache sync never finished", "name", rm.name)
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

func (rm *Manager) findRelatedParents(relateds ...*unstructured.Unstructured) []*unstructured.Unstructured {
	var matchingParents []*unstructured.Unstructured

	for _, parentInformer := range rm.parentInformers {
		parents, err := parentInformer.Lister().List(labels.Everything())
		if err != nil {
			return nil
		}

	MATCHPARENTS:
		for _, parent := range parents {
			customizeHookResponse := rm.GetCachedCustomizeHookResponse(parent)

			if customizeHookResponse == nil {
				continue
			}

			for _, relatedRule := range customizeHookResponse.RelatedResourceRules {
				for _, related := range relateds {
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
	if err != nil {
		return false, err
	}

	switch selectionType {
	case selectByLabels:
		selector, err := toSelector(relatedRule.LabelSelector)
		if err != nil {
			return false, err
		}
		return selector.Matches(labels.Set(related.GetLabels())), nil
	case selectByNamespaceAndNames:
		if parentResource.Namespaced {
			parentNamespace := parent.GetNamespace()
			if len(relatedRule.Namespace) != 0 && parentNamespace != relatedRule.Namespace {
				return false, fmt.Errorf("%s: Namespace of parent %s does not match with namespace %s of related rule for %s/%s", parentResource.Kind, parent.GetName(), relatedRule.Namespace, relatedRule.APIVersion, relatedRule.Resource)
			}
			if parentNamespace != related.GetNamespace() {
				return false, nil
			}
		}
		if len(relatedRule.Names) != 0 {
			relatedName := related.GetName()
			return stringInArray(relatedName, relatedRule.Names), nil
		}
		return true, nil
	}
	return false, fmt.Errorf("should not reach here")
}

func listObjects(selector labels.Selector, namespace string, informer *dynamicinformer.ResourceInformer) ([]*unstructured.Unstructured, error) {
	if len(namespace) != 0 {
		return informer.Lister().Namespace(namespace).List(selector)
	}
	return informer.Lister().List(selector)
}

func (rm *Manager) GetRelatedObjects(parent *unstructured.Unstructured) (common.ChildMap, error) {
	parentGroup, _ := schema.ParseGroupVersion(parent.GetAPIVersion())
	parentResource := rm.parentKinds.Get(schema.GroupKind{Group: parentGroup.Group, Kind: parent.GetKind()})
	if parentResource == nil {
		return nil, fmt.Errorf("unknown parent %v/%v", parentGroup, parent.GetKind())
	}

	parentNamespace := parent.GetNamespace()

	customizeHookResponse, err := rm.GetCustomizeHookResponse(parent)

	if err != nil {
		return nil, err
	}

	childMap := make(common.ChildMap)
	for _, relatedRule := range customizeHookResponse.RelatedResourceRules {
		relatedClient, informer, err := rm.getRelatedClient(relatedRule.APIVersion, relatedRule.Resource)
		if err != nil {
			return nil, err
		}

		selectionType, err := determineSelectionType(relatedRule)
		if err != nil {
			return nil, err
		}

		switch selectionType {
		case selectByLabels:
			selector, err := toSelector(relatedRule.LabelSelector)
			if err != nil {
				return nil, err
			}
			var all []*unstructured.Unstructured
			if parentResource.Namespaced {
				all, err = informer.Lister().Namespace(parentNamespace).List(selector)
			} else {
				all, err = informer.Lister().List(selector)
			}
			if err != nil {
				return nil, fmt.Errorf("can't list %v related objects: %w", relatedClient.Kind, err)
			}
			childMap.InitGroup(relatedRule.APIVersion, relatedClient.Kind)
			childMap.InsertAll(parent, all)

		case selectByNamespaceAndNames:
			if parentResource.Namespaced && len(relatedRule.Namespace) != 0 && parentNamespace != relatedRule.Namespace {
				return nil, fmt.Errorf("requested related object namespace %s differs from parent object namespace %s", relatedRule.Namespace, parentNamespace)
			}
			all, err := listObjects(labels.Everything(), relatedRule.Namespace, informer)
			if err != nil {
				return nil, fmt.Errorf("can't list %v related objects: %w", relatedClient.Kind, err)
			}
			childMap.InitGroup(relatedRule.APIVersion, relatedClient.Kind)
			if len(relatedRule.Names) == 0 {
				childMap.InsertAll(parent, all)
			} else {
				for _, obj := range all {
					if stringInArray(obj.GetName(), relatedRule.Names) {
						childMap.Insert(parent, obj)
					}
				}
			}
		}
	}
	return childMap, err
}
