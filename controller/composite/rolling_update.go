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
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/metacontroller/controller/common"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	dynamicdiscovery "k8s.io/metacontroller/dynamic/discovery"
	dynamicobject "k8s.io/metacontroller/dynamic/object"
)

func (pc *parentController) syncRollingUpdate(parentRevisions []*parentRevision, observedChildren common.ChildMap) error {
	// Reconcile the set of existing child claims in ControllerRevisions.
	claimed := pc.syncRevisionClaims(parentRevisions)

	// Give the latest revision any children it desires that aren't claimed yet,
	// or that don't need any changes to match the desired state.
	latest := parentRevisions[0]
	for gvk, objects := range latest.desiredChildMap {
		apiVersion, kind := common.ParseChildMapKey(gvk)
		// Ignore the API version, because the 'claimed' map is version-agnostic.
		apiGroup, _ := common.ParseAPIVersion(apiVersion)

		// Skip if rolling update isn't enabled for this child type.
		if !pc.updateStrategy.isRolling(apiGroup, kind) {
			continue
		}

		claimMap := claimed.getKind(apiGroup, kind)
		for name, desiredChild := range objects {
			pr, found := claimMap[name]
			if !found {
				// No revision claims this child, so give it to the latest revision.
				latest.addChild(apiGroup, kind, name)
				claimed.setParentRevision(apiGroup, kind, name, latest)
				continue
			}
			if pr == latest {
				// It's already owned by the latest revision. Nothing to do.
				continue
			}
			// This child is claimed by another revision, but if it already matches
			// the desired state in the latest revision, we can move it immediately.
			child := observedChildren.FindGroupKindName(apiGroup, kind, name)
			if child == nil {
				// The child wasn't observed, so we don't know if it'll match latest.
				continue
			}
			updated, err := common.ApplyUpdate(child, desiredChild)
			if err != nil {
				// We can't prove it'll be a no-op, so don't move it to latest.
				continue
			}
			if reflect.DeepEqual(child, updated) {
				// This will be a no-op update, so move it immediately instead of
				// waiting until the next sync. In addition to reducing unnecessary
				// ControllerRevision updates, this helps ensure that the overall sync
				// won't be a no-op, which would mean there's nothing changing that
				// would trigger a resync to continue the rollout.
				latest.addChild(apiGroup, kind, name)
				pr.removeChild(apiGroup, kind, name)
				claimed.setParentRevision(apiGroup, kind, name, latest)
			}
		}
	}

	// Look for the next child to update, if any.
	// We go one by one, in the order in which the controller returned them
	// in the latest sync hook result.
	for _, child := range latest.desiredChildList {
		apiGroup, _ := common.ParseAPIVersion(child.GetAPIVersion())
		kind := child.GetKind()
		name := child.GetName()

		// Skip if rolling update isn't enabled for this child type.
		if !pc.updateStrategy.isRolling(apiGroup, kind) {
			continue
		}

		// Look up which revision claims this child, if any.
		var pr *parentRevision
		if claimMap := claimed.getKind(apiGroup, kind); claimMap != nil {
			pr = claimMap[name]
		}

		// Move the first child that isn't in the latest revision to the latest.
		if pr != latest {
			// We only continue to push more children into the latest revision if all
			// the children already in the latest revision are happy, where "happy" is
			// defined by the statusChecks in each child type's updateStrategy.
			if err := pc.shouldContinueRolling(latest, observedChildren); err != nil {
				// Add status condition to explain what we're waiting for.
				updatedCondition := &dynamicobject.StatusCondition{
					Type:    "Updated",
					Status:  "False",
					Reason:  "RolloutWaiting",
					Message: err.Error(),
				}
				dynamicobject.SetCondition(latest.status, updatedCondition)
				return nil
			}

			latest.addChild(apiGroup, kind, name)
			// Remove it from all other revisions.
			for _, pr := range parentRevisions[1:] {
				pr.removeChild(apiGroup, kind, name)
			}

			// We've done our one move for this sync pass.
			// Add status condition to explain what we're doing next.
			updatedCondition := &dynamicobject.StatusCondition{
				Type:    "Updated",
				Status:  "False",
				Reason:  "RolloutProgressing",
				Message: fmt.Sprintf("updating %v %v", kind, name),
			}
			dynamicobject.SetCondition(latest.status, updatedCondition)
			return nil
		}
	}

	// Everything is already on the latest revision.
	updatedCondition := &dynamicobject.StatusCondition{
		Type:    "Updated",
		Status:  "True",
		Reason:  "OnLatestRevision",
		Message: fmt.Sprintf("latest ControllerRevision: %v", latest.revision.Name),
	}
	dynamicobject.SetCondition(latest.status, updatedCondition)
	return nil
}

func (pc *parentController) shouldContinueRolling(latest *parentRevision, observedChildren common.ChildMap) error {
	// We continue rolling only if all children claimed by the latest revision
	// are updated and were observed in a "happy" state, according to the
	// user-supplied, resource-specific status checks.
	for _, ck := range latest.revision.Children {
		strategy := pc.updateStrategy.get(ck.APIGroup, ck.Kind)
		if !isRollingStrategy(strategy) {
			// We don't need to check children that don't use rolling update.
			continue
		}

		for _, name := range ck.Names {
			child := observedChildren.FindGroupKindName(ck.APIGroup, ck.Kind, name)
			if child == nil {
				// We didn't observe this child at all, so it's not happy.
				return fmt.Errorf("missing child %v %v", ck.Kind, name)
			}
			// Is this child up-to-date with what the latest revision wants?
			// Apply the latest update to it and see if anything changes.
			update := latest.desiredChildMap.FindGroupKindName(ck.APIGroup, ck.Kind, name)
			updated, err := common.ApplyUpdate(child, update)
			if err != nil {
				return fmt.Errorf("can't check if child %v %v is updated: %v", ck.Kind, name, err)
			}
			if !reflect.DeepEqual(child, updated) {
				return fmt.Errorf("child %v %v is not updated yet", ck.Kind, name)
			}
			// For RollingInPlace, we should check ObservedGeneration (if possible)
			// before checking status, to make sure status reflects the latest spec.
			if strategy.Method == v1alpha1.ChildUpdateRollingInPlace {
				// Ideally every controller would support ObservedGeneration, but not
				// all do, so we have to ignore it if it's not present.
				if observedGeneration := dynamicobject.GetObservedGeneration(child.UnstructuredContent()); observedGeneration > 0 {
					// Ideally we would remember the Generation from our own last Update,
					// but we don't have a good place to persist that.
					// Instead, we compare with the latest Generation, which should be
					// fine as long as the object spec is not updated frequently.
					if observedGeneration < child.GetGeneration() {
						return fmt.Errorf("child %v %v with RollingInPlace update strategy hasn't observed latest spec", ck.Kind, name)
					}
				}
			}
			// Check the child status according to the updateStrategy.
			if err := childStatusCheck(&strategy.StatusChecks, child); err != nil {
				// If any child already on the latest revision fails the status check,
				// pause the rollout.
				return fmt.Errorf("child %v %v failed status check: %v", ck.Kind, name, err)
			}
		}
	}
	return nil
}

func (pc *parentController) syncRevisionClaims(parentRevisions []*parentRevision) childClaimMap {
	// The latest revision is always the first item.
	latest := parentRevisions[0]

	// Build a map for lookup from a child to the parentRevision that claims it.
	claimed := make(map[string]map[string]*parentRevision)

	for _, pr := range parentRevisions {
		children := make([]v1alpha1.ControllerRevisionChildren, 0, len(pr.revision.Children))

		for _, ck := range pr.revision.Children {
			if !pc.updateStrategy.isRolling(ck.APIGroup, ck.Kind) {
				// Remove claims for any child kinds that no longer use rolling update.
				continue
			}

			key := claimMapKey(ck.APIGroup, ck.Kind)
			names := make([]string, 0, len(ck.Names))

			for _, name := range ck.Names {
				// Remove claims for any children that the latest revision no longer desires.
				// Such children will be deleted immediately, so we can forget the claim.
				if latest.desiredChildMap.FindGroupKindName(ck.APIGroup, ck.Kind, name) == nil {
					continue
				}

				// Get the sub-map for this child kind.
				claimMap := claimed[key]
				if _, exists := claimMap[name]; exists {
					// Another revision already claimed this child, so drop it from here.
					// The only precedence rule we care about is that the latest revision
					// wins over any other, which is ensured by the fact that the latest
					// revision is always first in the list.
					continue
				}
				// Create a new sub-map if necessary.
				if claimMap == nil {
					claimMap = make(map[string]*parentRevision)
					claimed[key] = claimMap
				}
				claimMap[name] = pr
				names = append(names, name)
			}

			if len(names) == 0 {
				// Remove the whole child kind if there are no names left.
				continue
			}

			children = append(children, ck)
		}

		pr.revision.Children = children
	}
	return claimed
}

func childStatusCheck(checks *v1alpha1.ChildUpdateStatusChecks, child *unstructured.Unstructured) error {
	if checks == nil {
		// Nothing to check.
		return nil
	}

	for _, condCheck := range checks.Conditions {
		cond := dynamicobject.GetStatusCondition(child.UnstructuredContent(), condCheck.Type)
		if cond == nil {
			return fmt.Errorf("required condition type missing: %q", condCheck.Type)
		}
		if condCheck.Status != nil {
			if cond.Status != *condCheck.Status {
				return fmt.Errorf("%q condition status is %q (want %q)", condCheck.Type, cond.Status, *condCheck.Status)
			}
		}
		if condCheck.Reason != nil {
			if cond.Reason != *condCheck.Reason {
				return fmt.Errorf("%q condition reason is %q (want %q)", condCheck.Type, cond.Reason, *condCheck.Reason)
			}
		}
	}
	return nil
}

type updateStrategyMap map[string]*v1alpha1.CompositeControllerChildUpdateStrategy

func (m updateStrategyMap) GetMethod(apiGroup, kind string) v1alpha1.ChildUpdateMethod {
	strategy := m.get(apiGroup, kind)
	if strategy == nil || strategy.Method == "" {
		return v1alpha1.ChildUpdateOnDelete
	}
	return strategy.Method
}

func (m updateStrategyMap) get(apiGroup, kind string) *v1alpha1.CompositeControllerChildUpdateStrategy {
	return m[claimMapKey(apiGroup, kind)]
}

func (m updateStrategyMap) isRolling(apiGroup, kind string) bool {
	return isRollingStrategy(m.get(apiGroup, kind))
}

func (m updateStrategyMap) anyRolling() bool {
	for _, strategy := range m {
		if isRollingStrategy(strategy) {
			return true
		}
	}
	return false
}

func isRollingStrategy(strategy *v1alpha1.CompositeControllerChildUpdateStrategy) bool {
	if strategy == nil {
		// This child kind uses OnDelete (don't update at all).
		return false
	}
	switch strategy.Method {
	case v1alpha1.ChildUpdateRollingInPlace, v1alpha1.ChildUpdateRollingRecreate:
		return true
	}
	return false
}

func makeUpdateStrategyMap(resources *dynamicdiscovery.ResourceMap, cc *v1alpha1.CompositeController) (updateStrategyMap, error) {
	m := make(updateStrategyMap)
	for _, child := range cc.Spec.ChildResources {
		if child.UpdateStrategy != nil && child.UpdateStrategy.Method != v1alpha1.ChildUpdateOnDelete {
			// Map resource name to kind name.
			resource := resources.Get(child.APIVersion, child.Resource)
			if resource == nil {
				return nil, fmt.Errorf("can't find child resource %q in %v", child.Resource, child.APIVersion)
			}
			// Ignore API version.
			apiGroup, _ := common.ParseAPIVersion(child.APIVersion)
			key := claimMapKey(apiGroup, resource.Kind)
			m[key] = child.UpdateStrategy
		}
	}
	return m, nil
}
