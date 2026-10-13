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

package decorator

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dynamicinformer "metacontroller/pkg/dynamic/informer"
)

// cleanupParentFinalizers removes the finalizer of this controller from all
// parent objects that have it. Stop() calls it when the DecoratorController is
// deleted. Without this cleanup, the parent objects cannot be deleted, because
// no controller is left to remove the finalizer.
//
// The finalize hook is not called. The cleanup only logs an error; it does not
// try the parent again.
func (c *decoratorController) cleanupParentFinalizers(ctx context.Context) {
	c.logger.V(4).Info("Cleaning up finalizers on parent resources", "controller", c.dc.Name)

	c.parentInformers.ForEach(func(gvr schema.GroupVersionResource, informer *dynamicinformer.ResourceInformer) {
		all, err := informer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error(err, "Failed to list parent resources for finalizer cleanup", "resource", gvr)
			return
		}
		if len(all) == 0 {
			return
		}

		// ResourceMap uses the apiVersion (group/version) as the key, not
		// the group.
		resource := c.resources.Get(gvr.GroupVersion().String(), gvr.Resource)
		if resource == nil {
			c.logger.V(4).Info("No resource info found for parent, skipping finalizer cleanup", "resource", gvr)
			return
		}

		parentClient, err := c.dynClient.Kind(resource.APIVersion, resource.Kind)
		if err != nil {
			c.logger.Error(err, "Failed to get client for parent resource", "resource", gvr)
			return
		}

		for _, parent := range all {
			if !controllerutil.ContainsFinalizer(parent, c.finalizer.Name) {
				continue
			}
			// RemoveFinalizer tries again when there is an update conflict.
			if _, err := parentClient.Namespace(parent.GetNamespace()).RemoveFinalizer(ctx, parent, c.finalizer.Name); err != nil {
				if apierrors.IsNotFound(err) {
					// The parent is already deleted.
					continue
				}
				c.logger.Error(err, "Failed to remove finalizer from parent", "parent", parent.GetName(), "namespace", parent.GetNamespace())
				continue
			}
			c.logger.V(4).Info("Removed finalizer from parent resource", "controller", c.dc.Name, "parent", parent.GetName(), "namespace", parent.GetNamespace())
		}
	})
}
