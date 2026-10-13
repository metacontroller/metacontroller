/*
Copyright 2026 Metacontroller authors.

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
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// cleanupParentFinalizers removes the finalizer of this controller from all
// parent objects that have it. Stop() calls it when the CompositeController is
// deleted. Without this cleanup, the parent objects cannot be deleted, because
// no controller is left to remove the finalizer.
//
// The finalize hook is not called. The cleanup only logs an error; it does not
// try the parent again.
func (pc *parentController) cleanupParentFinalizers(ctx context.Context) {
	pc.logger.V(4).Info("Cleaning up finalizers on parent resources", "controller", pc.cc.Name)

	all, err := pc.parentInformer.Lister().List(labels.Everything())
	if err != nil {
		pc.logger.Error(err, "Failed to list parent resources for finalizer cleanup", "resource", pc.parentResource.Name)
		return
	}

	for _, parent := range all {
		if !controllerutil.ContainsFinalizer(parent, pc.finalizer.Name) {
			continue
		}
		// RemoveFinalizer tries again when there is an update conflict.
		if _, err := pc.parentClient.Namespace(parent.GetNamespace()).RemoveFinalizer(ctx, parent, pc.finalizer.Name); err != nil {
			if apierrors.IsNotFound(err) {
				// The parent is already deleted.
				continue
			}
			pc.logger.Error(err, "Failed to remove finalizer from parent", "parent", parent.GetName(), "namespace", parent.GetNamespace())
			continue
		}
		pc.logger.V(4).Info("Removed finalizer from parent resource", "controller", pc.cc.Name, "parent", parent.GetName(), "namespace", parent.GetNamespace())
	}
}
