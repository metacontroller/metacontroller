/*
Copyright 2018 The Kubernetes Authors.

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

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	scheme "k8s.io/metacontroller/client/generated/clientset/internalclientset/scheme"
)

// InitializerControllersGetter has a method to return a InitializerControllerInterface.
// A group's client should implement this interface.
type InitializerControllersGetter interface {
	InitializerControllers() InitializerControllerInterface
}

// InitializerControllerInterface has methods to work with InitializerController resources.
type InitializerControllerInterface interface {
	Create(*v1alpha1.InitializerController) (*v1alpha1.InitializerController, error)
	Update(*v1alpha1.InitializerController) (*v1alpha1.InitializerController, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.InitializerController, error)
	List(opts v1.ListOptions) (*v1alpha1.InitializerControllerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.InitializerController, err error)
	InitializerControllerExpansion
}

// initializerControllers implements InitializerControllerInterface
type initializerControllers struct {
	client rest.Interface
}

// newInitializerControllers returns a InitializerControllers
func newInitializerControllers(c *MetacontrollerV1alpha1Client) *initializerControllers {
	return &initializerControllers{
		client: c.RESTClient(),
	}
}

// Get takes name of the initializerController, and returns the corresponding initializerController object, and an error if there is any.
func (c *initializerControllers) Get(name string, options v1.GetOptions) (result *v1alpha1.InitializerController, err error) {
	result = &v1alpha1.InitializerController{}
	err = c.client.Get().
		Resource("initializercontrollers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of InitializerControllers that match those selectors.
func (c *initializerControllers) List(opts v1.ListOptions) (result *v1alpha1.InitializerControllerList, err error) {
	result = &v1alpha1.InitializerControllerList{}
	err = c.client.Get().
		Resource("initializercontrollers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested initializerControllers.
func (c *initializerControllers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("initializercontrollers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a initializerController and creates it.  Returns the server's representation of the initializerController, and an error, if there is any.
func (c *initializerControllers) Create(initializerController *v1alpha1.InitializerController) (result *v1alpha1.InitializerController, err error) {
	result = &v1alpha1.InitializerController{}
	err = c.client.Post().
		Resource("initializercontrollers").
		Body(initializerController).
		Do().
		Into(result)
	return
}

// Update takes the representation of a initializerController and updates it. Returns the server's representation of the initializerController, and an error, if there is any.
func (c *initializerControllers) Update(initializerController *v1alpha1.InitializerController) (result *v1alpha1.InitializerController, err error) {
	result = &v1alpha1.InitializerController{}
	err = c.client.Put().
		Resource("initializercontrollers").
		Name(initializerController.Name).
		Body(initializerController).
		Do().
		Into(result)
	return
}

// Delete takes name of the initializerController and deletes it. Returns an error if one occurs.
func (c *initializerControllers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("initializercontrollers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *initializerControllers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("initializercontrollers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched initializerController.
func (c *initializerControllers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.InitializerController, err error) {
	result = &v1alpha1.InitializerController{}
	err = c.client.Patch(pt).
		Resource("initializercontrollers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
