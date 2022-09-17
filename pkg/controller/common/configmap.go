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

func GetGvkFromConfigMap(
	dynClient *dynamicclientset.Clientset,
	parent *unstructured.Unstructured,
) ([]commonv1.GroupVersionKind, error) {

	var result []commonv1.GroupVersionKind
	client, err := dynClient.Kind("v1", "ConfigMap")
	if err != nil {
		return result, err
	}

	configMap, err := client.Namespace(parent.GetNamespace()).Get(context.TODO(), string(parent.GetUID()), metav1.GetOptions{})
	if err != nil {
		return result, nil
	}

	data := configMap.Object["data"].(map[string]interface{})
	resources := data["resources"].(string)

	err = json.Unmarshal([]byte(resources), &result)
	if err != nil {
		return result, err
	}

	return result, err
}

func UpdateConfigMap(
	dynClient *dynamicclientset.Clientset,
	parent *unstructured.Unstructured,
	desiredChildren commonv1.RelativeObjectMap,
) ([]commonv1.GroupVersionKind, error) {
	var result []commonv1.GroupVersionKind

	client, err := dynClient.Kind("v1", "ConfigMap")
	if err != nil {
		return result, err
	}

	var gvks = map[commonv1.GroupVersionKind]bool{}
	for key := range desiredChildren {
		gvks[key] = false
	}

	var observedChildren commonv1.RelativeObjectMap
	configMap, err := client.Namespace(parent.GetNamespace()).Get(context.TODO(), string(parent.GetUID()), metav1.GetOptions{})
	if err == nil {
		data := configMap.Object["data"].(map[string]interface{})
		resources := data["resources"].(string)

		var arr []commonv1.GroupVersionKind
		err = json.Unmarshal([]byte(resources), &arr)
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
		"resources": res,
	}

	var gvk = commonv1.GroupVersionKind{configMap.GroupVersionKind()}
	var objects = map[string]*unstructured.Unstructured{
		configMap.GetName(): configMap,
	}

	if err := updateChildren(client, inPlace{}, parent, observedChildren[gvk], objects); err != nil {
		return result, err
	}

	return result, nil
}
