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

package v1alpha1

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	v1alpha1 "k8s.io/metacontroller/apis/metacontroller/v1alpha1"
)

type ControllerRevisionExpansion interface {
	UpdateWithRetries(orig *v1alpha1.ControllerRevision, updateFn func(*v1alpha1.ControllerRevision) bool) (result *v1alpha1.ControllerRevision, err error)
}

func (c *controllerRevisions) UpdateWithRetries(orig *v1alpha1.ControllerRevision, updateFn func(*v1alpha1.ControllerRevision) bool) (result *v1alpha1.ControllerRevision, err error) {
	name := orig.GetName()
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		current, err := c.Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.GetUID() != orig.GetUID() {
			return apierrors.NewGone(fmt.Sprintf("can't update ControllerRevision %v/%v: original object is gone: got uid %v, want %v", orig.GetNamespace(), orig.GetName(), current.GetUID(), orig.GetUID()))
		}
		if changed := updateFn(current); !changed {
			// There's nothing to do.
			return nil
		}
		result, err = c.Update(current)
		return err
	})
	return result, err
}
