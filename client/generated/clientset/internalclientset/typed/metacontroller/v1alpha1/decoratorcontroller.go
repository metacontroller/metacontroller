/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "metacontroller.io/apis/metacontroller/v1alpha1"
	scheme "metacontroller.io/client/generated/clientset/internalclientset/scheme"
)

// DecoratorControllersGetter has a method to return a DecoratorControllerInterface.
// A group's client should implement this interface.
type DecoratorControllersGetter interface {
	DecoratorControllers() DecoratorControllerInterface
}

// DecoratorControllerInterface has methods to work with DecoratorController resources.
type DecoratorControllerInterface interface {
	Create(*v1alpha1.DecoratorController) (*v1alpha1.DecoratorController, error)
	Update(*v1alpha1.DecoratorController) (*v1alpha1.DecoratorController, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.DecoratorController, error)
	List(opts v1.ListOptions) (*v1alpha1.DecoratorControllerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DecoratorController, err error)
	DecoratorControllerExpansion
}

// decoratorControllers implements DecoratorControllerInterface
type decoratorControllers struct {
	client rest.Interface
}

// newDecoratorControllers returns a DecoratorControllers
func newDecoratorControllers(c *MetacontrollerV1alpha1Client) *decoratorControllers {
	return &decoratorControllers{
		client: c.RESTClient(),
	}
}

// Get takes name of the decoratorController, and returns the corresponding decoratorController object, and an error if there is any.
func (c *decoratorControllers) Get(name string, options v1.GetOptions) (result *v1alpha1.DecoratorController, err error) {
	result = &v1alpha1.DecoratorController{}
	err = c.client.Get().
		Resource("decoratorcontrollers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of DecoratorControllers that match those selectors.
func (c *decoratorControllers) List(opts v1.ListOptions) (result *v1alpha1.DecoratorControllerList, err error) {
	result = &v1alpha1.DecoratorControllerList{}
	err = c.client.Get().
		Resource("decoratorcontrollers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested decoratorControllers.
func (c *decoratorControllers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("decoratorcontrollers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a decoratorController and creates it.  Returns the server's representation of the decoratorController, and an error, if there is any.
func (c *decoratorControllers) Create(decoratorController *v1alpha1.DecoratorController) (result *v1alpha1.DecoratorController, err error) {
	result = &v1alpha1.DecoratorController{}
	err = c.client.Post().
		Resource("decoratorcontrollers").
		Body(decoratorController).
		Do().
		Into(result)
	return
}

// Update takes the representation of a decoratorController and updates it. Returns the server's representation of the decoratorController, and an error, if there is any.
func (c *decoratorControllers) Update(decoratorController *v1alpha1.DecoratorController) (result *v1alpha1.DecoratorController, err error) {
	result = &v1alpha1.DecoratorController{}
	err = c.client.Put().
		Resource("decoratorcontrollers").
		Name(decoratorController.Name).
		Body(decoratorController).
		Do().
		Into(result)
	return
}

// Delete takes name of the decoratorController and deletes it. Returns an error if one occurs.
func (c *decoratorControllers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("decoratorcontrollers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *decoratorControllers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("decoratorcontrollers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched decoratorController.
func (c *decoratorControllers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DecoratorController, err error) {
	result = &v1alpha1.DecoratorController{}
	err = c.client.Patch(pt).
		Resource("decoratorcontrollers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
