/*
Copyright 2022 Metacontroller authors.

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

package informer

import (
	"context"
	"fmt"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	"metacontroller/pkg/logging"
	"metacontroller/pkg/options"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubernetes "k8s.io/client-go/kubernetes/typed/core/v1"

	"os"
	"strings"
)

// ResourceTweakListOptionsFunc is a function that transforms a v1.ListOptions
type ResourceTweakListOptionsFunc func(*dynamicclientset.ResourceClient, *metav1.ListOptions)

type Selector struct {
	Resources []string `yaml:"resources,omitempty"` // `selectors` will be applied to the specified resources, if `nil` `selectors` will be applied to all resources
	Selectors []string `yaml:"selectors"`
}

type SharedInformerTweakListOptions struct {
	LabelSelectors []Selector `yaml:"labelSelectors,omitempty"`
	FieldSelectors []Selector `yaml:"fieldSelectors,omitempty"`
	Namespaces     []string   `yaml:"namespaces,omitempty"` // namespaces to include in list options, if `nil` include all namespaces in the cluster
}

// readListOptionsConfig  parse YAML file and return SharedInformerTweakListOptions
func readListOptionsConfig(configPath string) (*SharedInformerTweakListOptions, error) {
	yamlContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var tweakListOptions SharedInformerTweakListOptions
	err = yaml.Unmarshal(yamlContent, &tweakListOptions)
	if err != nil {
		return nil, err
	}
	return &tweakListOptions, nil
}

// GetNamespaceListOptions returns v1.ListOptions with a field selector that excludes namespaces not in `s.Namespaces`
func (s *SharedInformerTweakListOptions) GetNamespaceListOptions(clusterNamespaceList *corev1.NamespaceList) (*metav1.ListOptions, error) {
	namespaceListOptions, err := s.getNamespaceListOptions(clusterNamespaceList)
	if err != nil {
		return nil, err
	}
	return namespaceListOptions, nil
}

// GetSelectorListOptions returns v1.ListOptions with field selectors and label selectors
func (s *SharedInformerTweakListOptions) GetSelectorListOptions(resourceName string) *metav1.ListOptions {
	listOptions := &metav1.ListOptions{}
	for _, v := range s.FieldSelectors {
		if shouldAddSelector(resourceName, v) {
			listOptions.FieldSelector = joinSelectors(listOptions.FieldSelector, strings.Join(v.Selectors, ","))
		}
	}
	for _, v := range s.LabelSelectors {
		if shouldAddSelector(resourceName, v) {
			listOptions.LabelSelector = joinSelectors(listOptions.LabelSelector, strings.Join(v.Selectors, ","))
		}
	}
	return listOptions
}

// GetResourceTweakListOptionsFunc creates a function that transforms a v1.ListOptions
func GetResourceTweakListOptionsFunc(configuration options.Configuration) (ResourceTweakListOptionsFunc, error) {
	// the transformation to perform on v1.ListOptions is determined by `configuration` and results in a combination of
	// namespaces to exclude via field selectors, field selectors to apply to either all or specific resources, and
	// label selectors to apply to either all or specific resources

	configFile := configuration.ListOptionsConfigFilePath
	if configFile == "" {
		logging.Logger.V(5).Info("No list options config provided, no tweak list options will be applied")
		return nil, nil
	}

	tweakListOptions, err := readListOptionsConfig(configFile)
	if err != nil {
		return nil, err
	}

	namespaceListOptions := &metav1.ListOptions{}
	if tweakListOptions.Namespaces != nil {
		// do not rely on dynamic discovery because it is safe to assume that namespaces are already available in the cluster
		coreV1Client := kubernetes.NewForConfigOrDie(configuration.RestConfig)
		clusterNamespaceList, err := coreV1Client.Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		// list options with field selectors to exclude namespaces
		namespaceListOptions, err = tweakListOptions.GetNamespaceListOptions(clusterNamespaceList)
		if err != nil {
			return nil, err
		}
	}

	listOptionsFunc := func(resource *dynamicclientset.ResourceClient, options *metav1.ListOptions) {
		selectorListOptions := tweakListOptions.GetSelectorListOptions(resource.Name)
		fieldSelectors := selectorListOptions.FieldSelector
		if resource.Namespaced && tweakListOptions.Namespaces != nil { // do not filter by namespace on cluster scoped resources
			fieldSelectors = joinSelectors(namespaceListOptions.FieldSelector, fieldSelectors)
		}
		options.FieldSelector = joinSelectors(options.FieldSelector, fieldSelectors)
		options.LabelSelector = joinSelectors(options.LabelSelector, selectorListOptions.LabelSelector)
	}
	return listOptionsFunc, nil
}

func joinSelectors(selectors1, selectors2 string) string {
	if len(selectors1) == 0 {
		return selectors2
	}
	if len(selectors2) == 0 {
		return selectors1
	}
	return fmt.Sprintf("%s,%s", selectors1, selectors2)
}

// shouldAddSelector returns true if `selector.Resources` is `nil` or if `selector.Resources` contains `resource`
func shouldAddSelector(resource string, selector Selector) bool {
	if selector.Resources == nil { // if no resources are defined, the selector should be applied to all resources
		return true
	}
	for _, r := range selector.Resources {
		if r == resource {
			return true
		}
	}
	return false
}

func (s *SharedInformerTweakListOptions) getNamespaceListOptions(clusterNamespaceList *corev1.NamespaceList) (*metav1.ListOptions, error) {
	if s.Namespaces == nil {
		return &metav1.ListOptions{}, nil
	}

	var namespaces []string
	for _, v := range clusterNamespaceList.Items {
		namespaces = append(namespaces, v.GetName())
	}

	excludeNamespaces := difference(namespaces, s.Namespaces)
	var excluedNamespaceFieldSelectors []fields.Selector
	for _, v := range excludeNamespaces {
		excluedNamespaceFieldSelectors = append(excluedNamespaceFieldSelectors, fields.OneTermNotEqualSelector("metadata.namespace", v))
	}
	fieldSelector := fields.AndSelectors(excluedNamespaceFieldSelectors...)

	listOptions := &metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	}
	return listOptions, nil
}

// difference returns the elements in `a` that aren't in `b`. reference: https://stackoverflow.com/a/45428032/3499168
func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
