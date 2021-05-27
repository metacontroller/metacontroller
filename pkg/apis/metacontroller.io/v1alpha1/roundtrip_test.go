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

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	roundtrip "k8s.io/apimachinery/pkg/api/apitesting/roundtrip"
	metafuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// TestRoundTrip tests that the third-party kinds can be marshaled and unmarshaled correctly to/from JSON
// without the loss of information. Moreover, deep copy is tested.
func TestRoundTrip(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	AddToScheme(scheme)

	fuzzer := fuzzer.FuzzerFor(metafuzzer.Funcs, rand.NewSource(1), codecs)

	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("CompositeController"), scheme, codecs, fuzzer, nil)
	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("CompositeControllerList"), scheme, codecs, fuzzer, nil)
	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("DecoratorController"), scheme, codecs, fuzzer, nil)
	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("DecoratorControllerList"), scheme, codecs, fuzzer, nil)
	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("ControllerRevision"), scheme, codecs, fuzzer, nil)
	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("ControllerRevisionList"), scheme, codecs, fuzzer, nil)
}
