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

package cmd

import (
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/warpcomdev/think8shook/internal/webhook"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
)

func mustMarshal(v interface{}) json.RawMessage {
	marshal, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(marshal)
}

type podTestCase struct {
	Initial      map[string]json.RawMessage `json:"initial"`
	ShouldMutate bool                       `json:"shouldMutate"`
	Expected     []jsonPatch                `json:"expected"`
}

func TestSecurityPatches(t *testing.T) {
	var fset flag.FlagSet
	klog.InitFlags(&fset)
	fset.Parse([]string{
		"--v", "10",
		"--logtostderr", "true",
	})
	defer klog.Flush()
	mutator := podMutator(mutatePodSecurityContext, mutateContainerSecurityContext)
	err := filepath.WalkDir("pod_tests", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) != ".yaml" {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			// The yaml file provides pod and expected filter and mutation result.
			yamlFile, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var testCase podTestCase
			if err := yaml.Unmarshal(yamlFile, &testCase); err != nil {
				t.Fatal(err)
			}
			// Pod must be properly deserialized
			deserializer := webhook.Codecs().UniversalDeserializer()
			var pod corev1.Pod
			if _, _, err := deserializer.Decode(testCase.Initial["pod"], nil, &pod); err != nil {
				t.Fatal(err)
			}
			// Check if mutation filter matches
			mutated := shouldMutateSecurityContext(&pod)
			if mutated != testCase.ShouldMutate {
				t.Fatalf("expected mutated = %v, got %v", testCase.ShouldMutate, mutated)
			}
			// Test if mutations match
			ps := &patchSet{
				patches: make([]jsonPatch, 0, 16),
			}
			if err := mutator(&pod, ps); err != nil {
				t.Fatal(err)
			}
			mustEqual(t, ps, testCase.Expected)
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func mustEqual(t *testing.T, actual *patchSet, expected []jsonPatch) {
	if len(actual.patches) != len(expected) {
		t.Errorf("patch sets length do not match, got %s", actual)
		return
	}
	for i, a := range actual.patches {
		e := expected[i]
		if a.Op != e.Op {
			t.Errorf("Operations differ at position %d: expected %v, got %v", i, e.Op, a.Op)
			return
		}
		if a.Path != e.Path {
			t.Errorf("Paths differ at position %d: expected %v, got %v", i, e.Path, a.Path)
			return
		}
		require.JSONEq(t, string(e.Value), string(a.Value))
	}
}
