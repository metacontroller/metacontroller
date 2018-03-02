/*
Copyright 2018 Google Inc.

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

package composite

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/golang/glog"
	"k8s.io/metacontroller/controller/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	dynamiccontrollerref "k8s.io/metacontroller/dynamic/controllerref"
	k8s "k8s.io/metacontroller/third_party/kubernetes"
)

const (
	labelKeyAPIGroup = "metacontroller.k8s.io/apiGroup"
	labelKeyResource = "metacontroller.k8s.io/resource"
)

func (pc *parentController) claimRevisions(parent *unstructured.Unstructured) ([]*v1alpha1.ControllerRevision, error) {
	parentGVK := pc.parentResource.GroupVersionKind()

	// Add labels to prevent accidental overlap between different parent types.
	extraMatchLabels := map[string]string{
		labelKeyAPIGroup: pc.parentResource.Group,
		labelKeyResource: pc.parentResource.Name,
	}
	selector, err := pc.makeSelector(parent, extraMatchLabels)
	if err != nil {
		return nil, err
	}
	canAdoptFunc := pc.canAdoptFunc(parent)

	// List all ControllerRevisions in the parent object's namespace.
	all, err := pc.revisionLister.ControllerRevisions(parent.GetNamespace()).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("can't list ControllerRevisions: %v", err)
	}

	// Handle orphan/adopt and filter by owner+selector.
	client := pc.mcClient.MetacontrollerV1alpha1().ControllerRevisions(parent.GetNamespace())
	crm := dynamiccontrollerref.NewControllerRevisionManager(client, parent, selector, parentGVK, canAdoptFunc)
	revisions, err := crm.ClaimControllerRevisions(all)
	if err != nil {
		return nil, fmt.Errorf("can't claim ControllerRevisions: %v", err)
	}
	return revisions, nil
}

func (pc *parentController) syncRevisions(parent *unstructured.Unstructured, observedChildren common.ChildMap) (map[string]interface{}, common.ChildMap, error) {
	// If no child resources use rolling updates, just sync the latest parent.
	// If the parent object is being deleted, just sync the latest parent to get
	// the status; we don't manage children while being deleted anyway.
	if !pc.updateStrategy.anyRolling() || parent.GetDeletionTimestamp() != nil {
		syncRequest := &syncHookRequest{
			Controller: pc.cc,
			Parent:     parent,
			Children:   observedChildren,
		}
		syncResult, err := callSyncHook(pc.cc, syncRequest)
		if err != nil {
			return nil, nil, fmt.Errorf("sync hook failed for %v %v/%v: %v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
		}
		return syncResult.Status, common.MakeChildMap(syncResult.Children), nil
	}

	// Claim all matching ControllerRevisions for the parent.
	observedRevisions, err := pc.claimRevisions(parent)
	if err != nil {
		return nil, nil, err
	}

	// Extract the fields from parent that the controller author
	// said are relevant for revision history.
	// If nothing was specified, default to all of "spec".
	var fieldPaths []string
	if rh := pc.cc.Spec.ParentResource.RevisionHistory; rh != nil && len(rh.FieldPaths) > 0 {
		fieldPaths = rh.FieldPaths
	} else {
		fieldPaths = []string{"spec"}
	}
	latestPatch := makePatch(parent.UnstructuredContent(), fieldPaths)

	// The first item in the list is always the latest parent.
	// The rest are in no particular order.
	latest := &parentRevision{parent: parent}
	parentRevisions := make([]*parentRevision, 0, len(observedRevisions)+1)
	parentRevisions = append(parentRevisions, latest)

	// Materialize the parent object that each revision represents
	// by applying its parentPatch to the current parent object.
	// We make deep copies of the ControllerRevisions since we modify them later.
	for _, revision := range observedRevisions {
		patch := make(map[string]interface{})
		if err := json.Unmarshal(revision.ParentPatch.Raw, &patch); err != nil {
			return nil, nil, fmt.Errorf("can't unmarshal ControllerRevision parentPatch: %v", err)
		}
		if reflect.DeepEqual(patch, latestPatch) {
			// This ControllerRevision matches the latest parent state.
			latest.revision = revision.DeepCopy()
			continue
		}
		// Also deep copy parent, so we can apply the patch to it.
		pr := &parentRevision{parent: latest.parent.DeepCopy(), revision: revision.DeepCopy()}
		applyPatch(pr.parent.UnstructuredContent(), patch, fieldPaths)
		parentRevisions = append(parentRevisions, pr)
	}

	// Create a new ControllerRevision for the latest parent state, if needed.
	if latest.revision == nil {
		revision, err := newControllerRevision(&pc.parentResource.APIResource, latest.parent, latestPatch)
		if err != nil {
			return nil, nil, err
		}
		latest.revision = revision
	}

	// Call the sync hook to get each parent revision's idea of the desired children.
	var wg sync.WaitGroup
	for _, pr := range parentRevisions {
		wg.Add(1)
		go func(pr *parentRevision) {
			defer wg.Done()

			syncRequest := &syncHookRequest{
				Controller: pc.cc,
				Parent:     pr.parent,
				Children:   observedChildren,
			}
			syncResult, err := callSyncHook(pc.cc, syncRequest)
			if err != nil {
				pr.syncError = err
				return
			}
			pr.status = syncResult.Status
			pr.desiredChildList = syncResult.Children
			pr.desiredChildMap = common.MakeChildMap(syncResult.Children)
		}(pr)
	}
	wg.Wait()

	// If any of the sync calls failed, abort.
	for _, pr := range parentRevisions {
		if pr.syncError != nil {
			return nil, nil, fmt.Errorf("sync hook failed for %v %v/%v: %v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), pr.syncError)
		}
	}

	// Manipulate revisions to proceed with any ongoing rollout, if possible.
	if err := pc.syncRollingUpdate(parentRevisions, observedChildren); err != nil {
		return nil, nil, err
	}

	// Remove any ControllerRevisions that no longer have any children.
	// We don't remember previous revisions that we finished migrating away from.
	// The user is responsible for recovering an old config from source control
	// if a rollback is necessary.
	parentRevisions = pruneParentRevisions(parentRevisions)

	// Reconcile any changes to ControllerRevision objects.
	// For now, we require these changes to all commit before we start managing
	// children.
	// We don't want to start acting before we persist our desired end state.
	desiredRevisions := make([]*v1alpha1.ControllerRevision, 0, len(parentRevisions))
	for _, pr := range parentRevisions {
		if pr.revision != nil {
			desiredRevisions = append(desiredRevisions, pr.revision)
		}
	}
	if err := pc.manageRevisions(parent, observedRevisions, desiredRevisions); err != nil {
		return nil, nil, fmt.Errorf("%v %v/%v: can't reconcile ControllerRevisions: %v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
	}

	// We now know which revision ought to be responsible for which children.
	// Start with the latest revision's desired children.
	// Then overwrite any children that are still claimed by other revisions.
	desiredChildren := latest.desiredChildMap
	for _, pr := range parentRevisions[1:] {
		for _, ck := range pr.revision.Children {
			for _, name := range ck.Names {
				child := pr.desiredChildMap.FindGroupKindName(ck.APIGroup, ck.Kind, name)
				if child != nil {
					desiredChildren.ReplaceChild(child)
				}
			}
		}
	}

	// We only take parent status from the latest revision.
	return latest.status, desiredChildren, nil
}

func (pc *parentController) manageRevisions(parent *unstructured.Unstructured, observedRevisions, desiredRevisions []*v1alpha1.ControllerRevision) error {
	client := pc.mcClient.MetacontrollerV1alpha1().ControllerRevisions(parent.GetNamespace())

	// Build maps for convenient lookup by object name.
	observedMap := make(map[string]*v1alpha1.ControllerRevision, len(observedRevisions))
	desiredMap := make(map[string]*v1alpha1.ControllerRevision, len(desiredRevisions))
	for _, revision := range desiredRevisions {
		desiredMap[revision.Name] = revision
	}

	// Delete observed, owned objects that are not desired.
	for _, revision := range observedRevisions {
		observedMap[revision.Name] = revision

		if _, desired := desiredMap[revision.Name]; !desired {
			opts := &metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{UID: &revision.UID},
			}
			glog.Infof("%v %v/%v: deleting ControllerRevision %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), revision.GetName())
			if err := client.Delete(revision.Name, opts); err != nil {
				return fmt.Errorf("can't delete ControllerRevision %v for %v %v/%v: %v", revision.Name, pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
			}
		}
	}

	// Create or update desired objects.
	for _, revision := range desiredRevisions {
		if oldObj := observedMap[revision.Name]; oldObj != nil {
			// Update
			if reflect.DeepEqual(oldObj, revision) {
				// We didn't change anything.
				continue
			}
			glog.Infof("%v %v/%v: updating ControllerRevision %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), revision.GetName())
			if _, err := client.Update(revision); err != nil {
				return fmt.Errorf("can't update ControllerRevision %v for %v %v/%v: %v", revision.Name, pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
			}
		} else {
			// Create
			controllerRef := common.MakeControllerRef(parent)
			revision.OwnerReferences = append(revision.OwnerReferences, *controllerRef)
			glog.Infof("%v %v/%v: creating ControllerRevision %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), revision.GetName())
			if _, err := client.Create(revision); err != nil {
				return fmt.Errorf("can't create ControllerRevision %v for %v %v/%v: %v", revision.Name, pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
			}
		}
	}

	return nil
}

func newControllerRevision(parentResource *metav1.APIResource, parent *unstructured.Unstructured, patch map[string]interface{}) (*v1alpha1.ControllerRevision, error) {
	patchData, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("can't marshal ControllerRevision parentPatch: %v", err)
	}

	// Get labels from the parent object's spec.template.
	// This is how we find any orphaned ControllerRevisions for a given parent.
	labels := k8s.GetNestedMap(parent.UnstructuredContent(), "spec", "template", "metadata", "labels")

	// Add labels to prevent accidental overlap between different parent types.
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[labelKeyAPIGroup] = parentResource.Group
	labels[labelKeyResource] = parentResource.Name

	revision := &v1alpha1.ControllerRevision{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "ControllerRevision",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerRevisionName(parentResource, parent, patchData),
			Namespace: parent.GetNamespace(),
			Labels:    labels,
		},
		ParentPatch: runtime.RawExtension{Raw: patchData},
	}
	return revision, nil
}

func controllerRevisionName(parentResource *metav1.APIResource, parent *unstructured.Unstructured, patchData []byte) string {
	apiGroup := parentResource.Group
	if apiGroup == "" {
		apiGroup = "core"
	}
	// ControllerRevision names are not meant to be human-readable.
	// We could use just the hash, since it should be globally unique.
	// However, we prefix with the fully-qualified resource name to lend some
	// sanity to the listing in case anyone looks.
	prefix := fmt.Sprintf("%s.%s", parentResource.Name, apiGroup)
	// Make sure the name is 253 chars or less.
	// We need 40 for the hash, plus 1 for the separator.
	if len(prefix) > (253 - 41) {
		prefix = prefix[:(253 - 41)]
	}
	return fmt.Sprintf("%s-%s", prefix, controllerRevisionHash([]byte(parent.GetUID()), patchData))
}

func controllerRevisionHash(parentUID, patchData []byte) string {
	// We don't do collision avoidance, so use something
	// with very low accidental collision probability.
	hasher := sha1.New()
	// Add the parent UID since parent names can collide across resources.
	// It doesn't matter that the UID won't match after adoption.
	// This hash is only used for idempotent creation, not for lookup.
	hasher.Write([]byte(parentUID))
	hasher.Write(patchData)
	return hex.EncodeToString(hasher.Sum(nil))
}

func makePatch(src map[string]interface{}, fieldPaths []string) map[string]interface{} {
	patch := make(map[string]interface{})
	for _, fieldPath := range fieldPaths {
		pathParts := strings.Split(fieldPath, ".")
		if value := k8s.GetNestedField(src, pathParts...); value != nil {
			k8s.SetNestedField(patch, value, pathParts...)
		}
	}
	return patch
}

func applyPatch(dest, patch map[string]interface{}, fieldPaths []string) {
	for _, fieldPath := range fieldPaths {
		pathParts := strings.Split(fieldPath, ".")
		if value := k8s.GetNestedField(patch, pathParts...); value != nil {
			k8s.SetNestedField(dest, value, pathParts...)
		}
	}
}

type parentRevision struct {
	parent   *unstructured.Unstructured
	revision *v1alpha1.ControllerRevision

	status           map[string]interface{}
	desiredChildList []*unstructured.Unstructured
	desiredChildMap  common.ChildMap
	syncError        error
}

func (pr *parentRevision) countChildren() int {
	count := 0
	if pr.revision == nil {
		return count
	}
	for _, children := range pr.revision.Children {
		count += len(children.Names)
	}
	return count
}

func (pr *parentRevision) addChild(apiGroup, kind, name string) {
	// Find the matching group.
	var children *v1alpha1.ControllerRevisionChildren
	for i, ch := range pr.revision.Children {
		if ch.APIGroup == apiGroup && ch.Kind == kind {
			children = &pr.revision.Children[i]
			break
		}
	}
	// Start a new group if needed.
	if children == nil {
		pr.revision.Children = append(pr.revision.Children, v1alpha1.ControllerRevisionChildren{APIGroup: apiGroup, Kind: kind})
		children = &pr.revision.Children[len(pr.revision.Children)-1]
	}
	// If it's already in the list, there's nothing to do.
	for _, n := range children.Names {
		if n == name {
			return
		}
	}
	children.Names = append(children.Names, name)
}

func (pr *parentRevision) removeChild(apiGroup, kind, name string) {
	// Find the matching group.
	var children *v1alpha1.ControllerRevisionChildren
	for i, ch := range pr.revision.Children {
		if ch.APIGroup == apiGroup && ch.Kind == kind {
			children = &pr.revision.Children[i]
			break
		}
	}
	if children == nil {
		// The group doesn't exist, so the child can't be there. Nothing to do.
		return
	}
	// Find it in the list, if it's there.
	pos := -1
	for i, n := range children.Names {
		if n == name {
			pos = i
			break
		}
	}
	// If the name wasn't found, there's nothing to do.
	if pos < 0 {
		return
	}
	// Remove the item at the specified position.
	children.Names = append(children.Names[:pos], children.Names[pos+1:]...)
}

func pruneParentRevisions(parentRevisions []*parentRevision) []*parentRevision {
	result := make([]*parentRevision, 0, len(parentRevisions))
	// Always include the first item (the latest revision).
	result = append(result, parentRevisions[0])
	// Include the rest only if they have remaining children.
	for _, pr := range parentRevisions[1:] {
		if pr.countChildren() > 0 {
			result = append(result, pr)
		}
	}
	return result
}

type childClaimMap map[string]map[string]*parentRevision

func (m childClaimMap) getKind(apiGroup, kind string) map[string]*parentRevision {
	return m[claimMapKey(apiGroup, kind)]
}

func (m childClaimMap) setParentRevision(apiGroup, kind, name string, pr *parentRevision) {
	key := claimMapKey(apiGroup, kind)
	claimMap := m[key]
	if claimMap == nil {
		claimMap = make(map[string]*parentRevision)
		m[key] = claimMap
	}
	claimMap[name] = pr
}

func claimMapKey(apiGroup, kind string) string {
	return fmt.Sprintf("%s.%s", kind, apiGroup)
}
