/*
Copyright 2017 Google Inc.

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
	"math/rand"
	"testing"

	"github.com/google/gofuzz"
	"k8s.io/apimachinery/pkg/api/testing/fuzzer"
	roundtrip "k8s.io/apimachinery/pkg/api/testing/roundtrip"
	metafuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
)

var _ runtime.Object = &CompositeController{}
var _ metav1.ObjectMetaAccessor = &CompositeController{}

var _ runtime.Object = &CompositeControllerList{}
var _ metav1.ListMetaAccessor = &CompositeControllerList{}

func CompositeControllerFuzzerFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(obj *CompositeControllerList, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			obj.Items = make([]CompositeController, c.Intn(10))
			for i := range obj.Items {
				c.Fuzz(&obj.Items[i])
			}
		},
	}
}

// TestRoundTrip tests that the third-party kinds can be marshaled and unmarshaled correctly to/from JSON
// without the loss of information. Moreover, deep copy is tested.
func TestRoundTrip(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	AddToScheme(scheme)

	seed := rand.Int63()
	fuzzerFuncs := fuzzer.MergeFuzzerFuncs(metafuzzer.Funcs, CompositeControllerFuzzerFuncs)
	fuzzer := fuzzer.FuzzerFor(fuzzerFuncs, rand.NewSource(seed), codecs)

	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("CompositeController"), scheme, codecs, fuzzer, nil)
	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("CompositeControllerList"), scheme, codecs, fuzzer, nil)
}
