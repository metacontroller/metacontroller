package common

import (
	"context"
	"encoding/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	commonv1 "metacontroller/pkg/controller/common/api/v1"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
)

type inPlace struct{}

func (m inPlace) GetMethod(string, string) v1alpha1.ChildUpdateMethod {
	return v1alpha1.ChildUpdateInPlace
}

func GetGvkFromConfigMapCC(
	cc *v1alpha1.CompositeController,
	dynClient *dynamicclientset.Clientset,
	parent *unstructured.Unstructured,
) ([]commonv1.GroupVersionKind, error) {

	var result []commonv1.GroupVersionKind
	client, err := dynClient.Kind("v1", "ConfigMap")
	if err != nil {
		return result, err
	}

	configMap, err := client.Namespace(cc.Spec.DefaultUpdateStrategy.ConfigMapNamespace).Get(context.TODO(), string(parent.GetUID()), metav1.GetOptions{})
	if err != nil {
		return result, nil
	}

	data := configMap.Object["data"].(map[string]interface{})
	children := data["children"].(string)

	err = json.Unmarshal([]byte(children), &result)
	if err != nil {
		return result, err
	}

	return result, err
}

// UpdateConfigMapCC If a default update strategy other than OnDelete is enabled, a ConfigMap will be created with a list of GVK children
//that will be applied by the controller. In the future, this ConfigMap will be read to update the list of children
//and create informers dynamically.
func UpdateConfigMapCC(
	cc *v1alpha1.CompositeController,
	dynClient *dynamicclientset.Clientset,
	parent *unstructured.Unstructured,
	desiredChildren commonv1.RelativeObjectMap,
) ([]commonv1.GroupVersionKind, error) {
	var result []commonv1.GroupVersionKind
	client, err := dynClient.Kind("v1", "ConfigMap")
	if err != nil {
		return result, err
	}

	if cc.Spec.DefaultUpdateStrategy.Method != v1alpha1.ChildUpdateOnDelete {

		var gvks = map[commonv1.GroupVersionKind]bool{}
		for key := range desiredChildren {
			gvks[key] = false
		}

		var observedChildren commonv1.RelativeObjectMap
		configMap, err := client.Namespace(cc.Spec.DefaultUpdateStrategy.ConfigMapNamespace).Get(context.TODO(), string(parent.GetUID()), metav1.GetOptions{})
		if err == nil {
			data := configMap.Object["data"].(map[string]interface{})
			children := data["children"].(string)

			var arr []commonv1.GroupVersionKind
			err = json.Unmarshal([]byte(children), &arr)
			if err != nil {
				return result, err
			}
			for _, gvk := range arr {
				gvks[gvk] = false
			}

			observedChildren = commonv1.MakeRelativeObjectMap(
				configMap,
				[]*unstructured.Unstructured{configMap},
			)
		} else {
			observedChildren = commonv1.RelativeObjectMap{}
		}

		configMap = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": parent.GetUID(),
				},
				"data": map[string]interface{}{},
			},
		}

		for key := range gvks {
			result = append(result, key)
		}
		j, err := json.Marshal(result)
		if err != nil {
			return result, err
		}
		res := string(j)
		configMap.Object["data"] = map[string]interface{}{
			"children": res,
		}

		var gvk = commonv1.GroupVersionKind{configMap.GroupVersionKind()}
		var objects = map[string]*unstructured.Unstructured{
			configMap.GetName(): configMap,
		}

		if err := updateChildren(client, inPlace{}, parent, observedChildren[gvk], objects); err != nil {
			return result, err
		}

		return result, nil
	} else {
		configMap, err := client.Namespace(cc.Spec.DefaultUpdateStrategy.ConfigMapNamespace).Get(context.TODO(), string(parent.GetUID()), metav1.GetOptions{})
		if err != nil {
			return result, nil
		}
		var observedChildren commonv1.RelativeObjectMap
		observedChildren = commonv1.MakeRelativeObjectMap(
			configMap,
			[]*unstructured.Unstructured{configMap},
		)
		var objects = map[string]*unstructured.Unstructured{}
		var gvk = commonv1.GroupVersionKind{configMap.GroupVersionKind()}
		if err := deleteChildren(client, parent, observedChildren[gvk], objects); err != nil {
			return result, err
		}
		return result, nil
	}
}

// UpdateConfigMapDC If a default update strategy other than OnDelete is enabled, a ConfigMap will be created with a list of GVK children
//that will be applied by the controller. In the future, this ConfigMap will be read to update the list of children
//and create informers dynamically.
func UpdateConfigMapDC(
	dc *v1alpha1.DecoratorController,
	dynClient *dynamicclientset.Clientset,
	parent *unstructured.Unstructured,
	desiredChildren commonv1.RelativeObjectMap,
) ([]commonv1.GroupVersionKind, error) {
	var result []commonv1.GroupVersionKind
	client, err := dynClient.Kind("v1", "ConfigMap")
	if err != nil {
		return result, err
	}

	if dc.Spec.DefaultUpdateStrategy.Method != v1alpha1.ChildUpdateOnDelete {

		var gvks = map[commonv1.GroupVersionKind]bool{}
		for key := range desiredChildren {
			gvks[key] = false
		}

		var observedChildren commonv1.RelativeObjectMap
		configMap, err := client.Namespace(dc.Spec.DefaultUpdateStrategy.ConfigMapNamespace).Get(context.TODO(), string(dc.GetUID()), metav1.GetOptions{})
		if err == nil {
			data := configMap.Object["data"].(map[string]interface{})
			children := data["children"].(string)

			var arr []commonv1.GroupVersionKind
			err = json.Unmarshal([]byte(children), &arr)
			if err != nil {
				return result, err
			}
			for _, gvk := range arr {
				gvks[gvk] = false
			}

			observedChildren = commonv1.MakeRelativeObjectMap(
				configMap,
				[]*unstructured.Unstructured{configMap},
			)
		} else {
			observedChildren = commonv1.RelativeObjectMap{}
		}

		configMap = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": dc.GetUID(),
				},
				"data": map[string]interface{}{},
			},
		}

		for key := range gvks {
			result = append(result, key)
		}
		j, err := json.Marshal(result)
		if err != nil {
			return result, err
		}
		res := string(j)
		configMap.Object["data"] = map[string]interface{}{
			"children": res,
		}

		var gvk = commonv1.GroupVersionKind{configMap.GroupVersionKind()}
		var objects = map[string]*unstructured.Unstructured{
			configMap.GetName(): configMap,
		}

		if err := updateChildren(client, inPlace{}, parent, observedChildren[gvk], objects); err != nil {
			return result, err
		}

		return result, nil
	} else {
		configMap, err := client.Namespace(dc.Spec.DefaultUpdateStrategy.ConfigMapNamespace).Get(context.TODO(), string(dc.GetUID()), metav1.GetOptions{})
		if err != nil {
			return result, nil
		}
		var observedChildren commonv1.RelativeObjectMap
		observedChildren = commonv1.MakeRelativeObjectMap(
			configMap,
			[]*unstructured.Unstructured{configMap},
		)
		var objects = map[string]*unstructured.Unstructured{}
		var gvk = commonv1.GroupVersionKind{configMap.GroupVersionKind()}
		if err := deleteChildren(client, parent, observedChildren[gvk], objects); err != nil {
			return result, err
		}
		return result, nil
	}
}

func GetGvkFromConfigMapDC(
	dc *v1alpha1.DecoratorController,
	dynClient *dynamicclientset.Clientset,
) ([]commonv1.GroupVersionKind, error) {

	var result []commonv1.GroupVersionKind
	client, err := dynClient.Kind("v1", "ConfigMap")
	if err != nil {
		return result, err
	}

	configMap, err := client.Namespace(dc.Spec.DefaultUpdateStrategy.ConfigMapNamespace).Get(context.TODO(), string(dc.GetUID()), metav1.GetOptions{})
	if err != nil {
		return result, nil
	}

	data := configMap.Object["data"].(map[string]interface{})
	children := data["children"].(string)

	err = json.Unmarshal([]byte(children), &result)
	if err != nil {
		return result, err
	}

	return result, err
}
