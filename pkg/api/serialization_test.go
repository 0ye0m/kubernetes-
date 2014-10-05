/*
Copyright 2014 Google Inc. All rights reserved.

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

package api_test

import (
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"reflect"
	"strconv"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta2"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/fsouza/go-dockerclient"
	"github.com/google/gofuzz"
)

var fuzzIters = flag.Int("fuzz_iters", 40, "How many fuzzing iterations to do.")

// apiObjectFuzzer can randomly populate api objects.
var apiObjectFuzzer = fuzz.New().NilChance(.5).NumElements(1, 1).Funcs(
	func(j *runtime.PluginBase, c fuzz.Continue) {
		// Do nothing; this struct has only a Kind field and it must stay blank in memory.
	},
	func(j *api.TypeMeta, c fuzz.Continue) {
		// We have to customize the randomization of TypeMeta because their
		// APIVersion and Kind must remain blank in memory.
		j.APIVersion = ""
		j.Kind = ""
	},
	func(j *api.ListMeta, c fuzz.Continue) {
		j.SelfLink = c.RandString()
		j.ResourceVersion = strconv.FormatUint(c.RandUint64()%(0x1<<31), 10) // JSON does not support high precision uints
	},
	func(j *api.ObjectMeta, c fuzz.Continue) {
		j.Namespace = c.RandString()
		j.Name = c.RandString()
		j.UID = c.RandString()
		j.SelfLink = c.RandString()
		j.ResourceVersion = strconv.FormatUint(c.RandUint64()%(0x1<<31), 10) // JSON does not support high precision uints

		var sec, nsec int64
		c.Fuzz(&sec)
		c.Fuzz(&nsec)
		j.CreationTimestamp = util.Unix(sec, nsec).Rfc3339Copy()

		c.Fuzz(&j.Labels)
		c.Fuzz(&j.Annotations)
	},
	func(j *v1beta1.JSONBase, c fuzz.Continue) {
		// We have to customize the randomization of TypeMeta because their
		// APIVersion and Kind must remain blank in memory.
		j.APIVersion = ""
		j.Kind = ""

		j.Namespace = c.RandString()
		j.ID = c.RandString()
		j.UID = c.RandString()
		j.SelfLink = c.RandString()
		j.ResourceVersion = c.RandUint64() % (0x1 << 31) // JSON does not support high precision uints

		var sec, nsec int64
		c.Fuzz(&sec)
		c.Fuzz(&nsec)
		j.CreationTimestamp = util.Unix(sec, nsec).Rfc3339Copy()
	},
	func(j *v1beta2.JSONBase, c fuzz.Continue) {
		// We have to customize the randomization of TypeMeta because their
		// APIVersion and Kind must remain blank in memory.
		j.APIVersion = ""
		j.Kind = ""

		j.Namespace = c.RandString()
		j.ID = c.RandString()
		j.UID = c.RandString()
		j.SelfLink = c.RandString()
		j.ResourceVersion = c.RandUint64() % (0x1 << 31) // JSON does not support high precision uints

		var sec, nsec int64
		c.Fuzz(&sec)
		c.Fuzz(&nsec)
		j.CreationTimestamp = util.Unix(sec, nsec).Rfc3339Copy()
	},
	func(j *v1beta1.ContainerManifest, c fuzz.Continue) {
		j.Version = "v1beta2"
		j.ID = c.RandString()
		j.UUID = c.RandString()
		c.Fuzz(&j.Volumes)
		c.Fuzz(&j.Containers)
		c.Fuzz(&j.RestartPolicy)
	},
	func(intstr *util.IntOrString, c fuzz.Continue) {
		// util.IntOrString will panic if its kind is set wrong.
		if c.RandBool() {
			intstr.Kind = util.IntstrInt
			intstr.IntVal = int(c.RandUint64())
			intstr.StrVal = ""
		} else {
			intstr.Kind = util.IntstrString
			intstr.IntVal = 0
			intstr.StrVal = c.RandString()
		}
	},
	func(u64 *uint64, c fuzz.Continue) {
		// TODO: uint64's are NOT handled right.
		*u64 = c.RandUint64() >> 8
	},
	func(pb map[docker.Port][]docker.PortBinding, c fuzz.Continue) {
		// This is necessary because keys with nil values get omitted.
		// TODO: Is this a bug?
		pb[docker.Port(c.RandString())] = []docker.PortBinding{
			{c.RandString(), c.RandString()},
			{c.RandString(), c.RandString()},
		}
	},
	func(pm map[string]docker.PortMapping, c fuzz.Continue) {
		// This is necessary because keys with nil values get omitted.
		// TODO: Is this a bug?
		pm[c.RandString()] = docker.PortMapping{
			c.RandString(): c.RandString(),
		}
	},
)

func clearGenericMetadataObject(obj interface{}) {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	m := v.FieldByName("Metadata").Addr().Interface().(*api.ObjectMeta)
	m.Annotations = nil
	m.Labels = nil
}

func clearGenericList(obj interface{}) {
	item := obj.(runtime.Object)
	children, err := runtime.ExtractList(item)
	if err != nil {
		panic(err)
	}
	for i := range children {
		t := reflect.TypeOf(children[i])
		if reset, ok := resetFunctions[t]; ok {
			reset(children[i])
		}
	}
}

func clearV1Beta1Pod(obj interface{}) {
	pod := obj.(*v1beta1.Pod)
	pod.DesiredState.Host = ""
	pod.DesiredState.HostIP = ""
	pod.DesiredState.Status = ""
	pod.DesiredState.PodIP = ""
	pod.DesiredState.Info = nil
	pod.DesiredState.Manifest.ID = pod.ID
	pod.DesiredState.Manifest.UUID = pod.UID // flawed
	for j := range pod.DesiredState.Manifest.Containers {
		container := &pod.DesiredState.Manifest.Containers[j]
		for k := range container.VolumeMounts {
			mount := &container.VolumeMounts[k]
			mount.Path = mount.MountPath
			mount.MountType = ""
		}
		for k := range container.Env {
			env := &container.Env[k]
			env.Key = env.Name
		}
	}
	pod.CurrentState.Manifest = v1beta1.ContainerManifest{}
}

var resetFunctions map[interface{}]func(interface{})

func init() {
	resetFunctions = map[interface{}]func(interface{}){
		reflect.TypeOf(&api.OperationList{}): clearGenericList,
		reflect.TypeOf(&api.Operation{}):     clearGenericMetadataObject,
		reflect.TypeOf(&v1beta1.PodList{}):   clearGenericList,
		reflect.TypeOf(&v1beta1.Pod{}):       clearV1Beta1Pod,
	}
}

func fuzzItem(item interface{}) {
	apiObjectFuzzer.Fuzz(item)
	t := reflect.TypeOf(item)
	if reset, ok := resetFunctions[t]; ok {
		reset(item)
	}
}

func setVersion(t *testing.T, source runtime.Object, kind, version string) runtime.Object {
	m, err := meta.FindObjectMeta(source)
	if err != nil {
		t.Fatalf("Unexpected error %v for %#v", err, source)
	}
	// ensure kind and version are set
	m.SetKind(kind)
	m.SetAPIVersion(version)
	return source
}

func runTest(t *testing.T, codec runtime.Codec, source runtime.Object, expectedKind, expectedVersion string) {
	name := reflect.TypeOf(source).Elem().String()

	data, err := codec.Encode(source)
	if err != nil {
		t.Errorf("%v: %v (%#v)", name, err, source)
		return
	}

	obj2, err := codec.Decode(data)
	if err != nil {
		t.Errorf("%v: %v", name, err)
		return
	}
	if !reflect.DeepEqual(source, obj2) {
		t.Errorf("1: %v: diff: %v", name, runtime.ObjectDiff(source, obj2))
		return
	}

	target := reflect.New(reflect.TypeOf(source).Elem()).Interface().(runtime.Object)
	err = codec.DecodeInto(data, target)
	if err != nil {
		t.Errorf("2: %v: %v", name, err)
		return
	}
	setVersion(t, target, expectedKind, expectedVersion)
	if !reflect.DeepEqual(source, target) {
		t.Errorf("3: %v: diff: %v", name, runtime.ObjectDiff(source, target))
		return
	}
}

func TestSpecificKind(t *testing.T) {
	for i := 0; i < *fuzzIters; i++ {
		codec := v1beta1.Codec
		item := &v1beta1.PodList{}
		kind := "PodList"
		fuzzItem(item)
		// The external API may have these attributes, but the internal does not.
		item.Namespace = ""
		item.ID = ""
		item.UID = ""
		setVersion(t, item, kind, "v1beta1")
		data, err := codec.Encode(item)
		if err != nil {
			t.Errorf("unable to encode: %v", err)
			return
		}

		obj2, err := codec.Decode(data)
		if err != nil {
			t.Errorf("unable to decode: %v", err)
			return
		}

		target := &v1beta1.PodList{}
		if err := api.Scheme.Convert(obj2, target); err != nil {
			t.Errorf("unable to convert: %v", err)
			return
		}
		if !reflect.DeepEqual(item, target) {
			t.Errorf("1: %v: diff: %v", kind, runtime.ObjectDiff(item, target))
			return
		}
	}
}

type logger struct{}

func (logger) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func TestTypeRename(t *testing.T) {
	for i := 0; i < *fuzzIters; i++ {
		item := &api.OperationList{}
		fuzzItem(item)
		runTest(t, v1beta1.Codec, item, "", "")
		runTest(t, v1beta2.Codec, item, "", "")
	}
}

func TestSimpleDecode(t *testing.T) {
	source := []byte(`{"kind":"PodList","apiVersion":"v1beta1","resourceVersion":42,"items":[{"resourceVersion":10}]}`)
	obj, err := api.Codec.Decode(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := obj.(*api.PodList)
	if !ok {
		t.Fatalf("not a PodList: %#v", obj)
	}
	if list.Metadata.ResourceVersion != "42" {
		t.Errorf("unexpected object: %#v", list)
	}
	if len(list.Items) != 1 || list.Items[0].Metadata.ResourceVersion != "10" {
		t.Errorf("unexpected object: %#v", list)
	}
	if _, err := v1beta1.Codec.Encode(list); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

/*
func TestPartialRoundTripTypes(t *testing.T) {
	for kind := range api.Scheme.KnownTypes("") {
		// Try a few times, since runTest uses random values.
		for i := 0; i < *fuzzIters; i++ {
			item, err := api.Scheme.New("", kind)
			if err != nil {
				t.Errorf("Couldn't make a %v? %v", kind, err)
				continue
			}
			apiObjectFuzzer.Fuzz(item)
			setVersion(t, item, kind, "")
			runTest(t, v1beta1.Codec, item, "", "")
			runTest(t, v1beta2.Codec, item, "", "")
		}
	}
}
*/
func TestFullRoundTripTypes(t *testing.T) {
	for kind := range api.Scheme.KnownTypes("") {
		// Try a few times, since runTest uses random values.
		for i := 0; i < *fuzzIters; i++ {
			item, err := api.Scheme.New("", kind)
			if err != nil {
				t.Errorf("Couldn't make a %v? %v", kind, err)
				continue
			}
			apiObjectFuzzer.Fuzz(item)
			setVersion(t, item, "", "")
			runTest(t, v1beta3.Codec, item, "", "")
			runTest(t, api.Codec, item, "", "")
		}
	}
}

func TestEncode_Ptr(t *testing.T) {
	pod := &api.Pod{
		Metadata: api.ObjectMeta{
			Labels: map[string]string{"name": "foo"},
		},
	}
	obj := runtime.Object(pod)
	data, err := latest.Codec.Encode(obj)
	obj2, err2 := latest.Codec.Decode(data)
	if err != nil || err2 != nil {
		t.Fatalf("Failure: '%v' '%v'", err, err2)
	}
	if _, ok := obj2.(*api.Pod); !ok {
		t.Fatalf("Got wrong type")
	}
	if !reflect.DeepEqual(obj2, pod) {
		t.Errorf("Expected:\n %#v,\n Got:\n %#v", pod, obj2)
	}
}

func TestBadJSONRejection(t *testing.T) {
	badJSONMissingKind := []byte(`{ }`)
	if _, err := latest.Codec.Decode(badJSONMissingKind); err == nil {
		t.Errorf("Did not reject despite lack of kind field: %s", badJSONMissingKind)
	}
	badJSONUnknownType := []byte(`{"kind": "bar"}`)
	if _, err1 := latest.Codec.Decode(badJSONUnknownType); err1 == nil {
		t.Errorf("Did not reject despite use of unknown type: %s", badJSONUnknownType)
	}
	/*badJSONKindMismatch := []byte(`{"kind": "Pod"}`)
	if err2 := DecodeInto(badJSONKindMismatch, &Minion{}); err2 == nil {
		t.Errorf("Kind is set but doesn't match the object type: %s", badJSONKindMismatch)
	}*/
}

const benchmarkSeed = 100

func BenchmarkEncode(b *testing.B) {
	pod := api.Pod{}
	apiObjectFuzzer.RandSource(rand.NewSource(benchmarkSeed))
	apiObjectFuzzer.Fuzz(&pod)
	for i := 0; i < b.N; i++ {
		latest.Codec.Encode(&pod)
	}
}

// BenchmarkEncodeJSON provides a baseline for regular JSON encode performance
func BenchmarkEncodeJSON(b *testing.B) {
	pod := api.Pod{}
	apiObjectFuzzer.RandSource(rand.NewSource(benchmarkSeed))
	apiObjectFuzzer.Fuzz(&pod)
	for i := 0; i < b.N; i++ {
		json.Marshal(&pod)
	}
}

func BenchmarkDecode(b *testing.B) {
	pod := api.Pod{}
	apiObjectFuzzer.RandSource(rand.NewSource(benchmarkSeed))
	apiObjectFuzzer.Fuzz(&pod)
	data, _ := latest.Codec.Encode(&pod)
	for i := 0; i < b.N; i++ {
		latest.Codec.Decode(data)
	}
}

func BenchmarkDecodeInto(b *testing.B) {
	pod := api.Pod{}
	apiObjectFuzzer.RandSource(rand.NewSource(benchmarkSeed))
	apiObjectFuzzer.Fuzz(&pod)
	data, _ := latest.Codec.Encode(&pod)
	for i := 0; i < b.N; i++ {
		obj := api.Pod{}
		latest.Codec.DecodeInto(data, &obj)
	}
}

// BenchmarkDecodeJSON provides a baseline for regular JSON decode performance
func BenchmarkDecodeJSON(b *testing.B) {
	pod := api.Pod{}
	apiObjectFuzzer.RandSource(rand.NewSource(benchmarkSeed))
	apiObjectFuzzer.Fuzz(&pod)
	data, _ := latest.Codec.Encode(&pod)
	for i := 0; i < b.N; i++ {
		obj := api.Pod{}
		json.Unmarshal(data, &obj)
	}
}
