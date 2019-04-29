package related

import (
	"fmt"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	v1alpha1 "metacontroller.app/apis/metacontroller/v1alpha1"
	"metacontroller.app/controller/common"
	dynamicclientset "metacontroller.app/dynamic/clientset"
	dynamicinformer "metacontroller.app/dynamic/informer"
	k8s "metacontroller.app/third_party/kubernetes"
)

type Manager struct {
	name             string
	metacontroller   common.CustomizableController

	interestedResourceKinds common.GroupKindMap

	dynClient        *dynamicclientset.Clientset
	dynInformers     *dynamicinformer.SharedInformerFactory
	parentInformer   *dynamicinformer.ResourceInformer

	relatedInformers common.InformerMap
	customizeCache   CustomizeResponseCache

	stopCh           chan struct{}

	enqueueParent    func(interface{})
}

func NewRelatedManager(
	name           string,
	enqueueParent  func(interface{}),
	metacontroller common.CustomizableController,
	dynClient      *dynamicclientset.Clientset,
	dynInformers   *dynamicinformer.SharedInformerFactory,
	parentInformer *dynamicinformer.ResourceInformer,
	interestedResourceKinds common.GroupKindMap,
) Manager {
	return Manager{
		name:                    name,
		metacontroller:          metacontroller,
		interestedResourceKinds: interestedResourceKinds,
		customizeCache:          make(CustomizeResponseCache),
		dynClient:               dynClient,
		dynInformers:            dynInformers,
		parentInformer:          parentInformer,
		relatedInformers:        make(common.InformerMap),
		enqueueParent:           enqueueParent,
	}
}

func (rm *Manager) Start(stopCh chan struct{}) {
	rm.stopCh = stopCh
}

func (rm *Manager) GetCachedCustomizeHookResponse(parent *unstructured.Unstructured) *common.CustomizeHookResponse {
	return rm.customizeCache.Get(parent.GetName(), parent.GetGeneration())
}

func (rm *Manager) GetCustomizeHookResponse(parent *unstructured.Unstructured) (*common.CustomizeHookResponse, error) {
	cached := rm.GetCachedCustomizeHookResponse(parent)
	if cached != nil {
		return cached, nil
	} else {
		response, err := common.CallCustomizeHook(rm.metacontroller, &common.CustomizeHookRequest{
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

	informer := rm.relatedInformers.Get(apiVersion, resource)
	if informer == nil {
		informer, err = rm.dynInformers.Resource(apiVersion, resource)

		if err != nil {
			return nil, nil, fmt.Errorf("can't create informer for related resource: %v", err)
		}

		informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    rm.onRelatedAdd,
			UpdateFunc: rm.onRelatedUpdate,
			DeleteFunc: rm.onRelatedDelete,
		})

		if !k8s.WaitForCacheSync(rm.name, rm.stopCh, informer.Informer().HasSynced) {
			glog.Warningf("related Manager %s cache sync never finished", rm.name)
		}

		rm.relatedInformers.Set(apiVersion, resource, informer)
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
	parents, err := rm.parentInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil
	}

	var matchingParents []*unstructured.Unstructured
	MATCHPARENTS:
	for _, parent := range parents {
		// TODO: We shouldn't call the customize hook here, but use cached results
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
	return matchingParents
}

func (rm *Manager) matchesRelatedRule(parent, related *unstructured.Unstructured, relatedRule *v1alpha1.RelatedResourceRule) (bool, error) {
	parentGroup, _ := common.ParseAPIVersion(parent.GetAPIVersion())
	parentResource := rm.interestedResourceKinds.Get(parentGroup, parent.GetKind())
	if parentResource == nil {
		return false, fmt.Errorf("Unknown parent %v/%v", parentGroup, parent.GetKind())
	}
	if parentResource.Namespaced {
		parentNamespace := parent.GetNamespace()
		if len(relatedRule.Namespace) != 0 && parentNamespace != relatedRule.Namespace {
			return false, fmt.Errorf("%s: Namespace of parent %s does not match with namespace %s of related rule for %s/%s", parentResource.Kind, parent.GetName(), relatedRule.Namespace, relatedRule.APIVersion, relatedRule.Resource)
		}
		if parentNamespace != related.GetNamespace() {
			return false, nil
		}
	}
	selector, err := metav1.LabelSelectorAsSelector(relatedRule.LabelSelector)
	if err != nil {
		return false, err
	}
	if !selector.Matches(labels.Set(related.GetLabels())) {
		return false, nil
	}
	if len(relatedRule.Names) != 0 {
		relatedName := related.GetName()
		for _, name := range relatedRule.Names {
			if name == relatedName {
				return true, nil
			}
		}
		return false, nil
	} else {
		return true, nil
	}
}

func (rm *Manager) GetRelatedObjects(parent *unstructured.Unstructured) (common.ChildMap, error) {

	parentGroup, _ := common.ParseAPIVersion(parent.GetAPIVersion())
	parentResource := rm.interestedResourceKinds.Get(parentGroup, parent.GetKind())
	if parentResource == nil {
		return nil, fmt.Errorf("Unknown parent %v/%v", parentGroup, parent.GetKind())
	}

	parentNamespace := parent.GetNamespace()

	customizeHookResponse, err := rm.GetCustomizeHookResponse(parent)

	if err != nil {
		return nil, err
	}

	childMap := make(common.ChildMap)
	for _, relatedRule := range customizeHookResponse.RelatedResourceRules {
		relatedClient, informer, err := rm.getRelatedClient(relatedRule.APIVersion, relatedRule.Resource)

		var selector labels.Selector
		if relatedRule.LabelSelector == nil {
			selector = labels.Everything()
		} else {
			selector, err = metav1.LabelSelectorAsSelector(relatedRule.LabelSelector)
			if err != nil {
				return nil, err
			}
		}

		var all []*unstructured.Unstructured
		if parentResource.Namespaced {
			if len(relatedRule.Namespace) != 0 && relatedRule.Namespace != parentNamespace {
				return nil, fmt.Errorf("requested related object namespace %s differs from parent object namespace %s", relatedRule.Namespace, parentNamespace)
			}
			all, err = informer.Lister().ListNamespace(parentNamespace, selector)
		} else if len(relatedRule.Namespace) != 0 {
			all, err = informer.Lister().ListNamespace(relatedRule.Namespace, selector)
		} else {
			all, err = informer.Lister().List(selector)
		}
		if err != nil {
			return nil, fmt.Errorf("can't list %v related objects: %v", relatedClient.Kind, relatedRule.Resource, err)
		}

		childMap.InitGroup(relatedRule.APIVersion, relatedClient.Kind)

		for _, obj := range all {
			if len(relatedRule.Names) == 0 {
				childMap.Insert(parent, obj)
			} else {
				for _, name := range relatedRule.Names {
					if name == obj.GetName() {
						childMap.Insert(parent, obj)
						break
					}
				}
			}
		}
	}

	return childMap, err
}
