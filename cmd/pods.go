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
	"fmt"

	"github.com/warpcomdev/think8shook/internal/webhook"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

const (
	InjectedUID = 1000
)

type jsonPatch struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

type patchSet struct {
	patches []jsonPatch
}

func (ps *patchSet) append(op, path string, value interface{}) error {
	marshal, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ps.patches = append(ps.patches, jsonPatch{
		Op:    op,
		Path:  path,
		Value: marshal,
	})
	return nil
}

func (ps patchSet) Json() (json.RawMessage, error) {
	marshal, err := json.Marshal(ps.patches)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(marshal), nil
}

func (ps patchSet) String() string {
	marshal, _ := ps.Json()
	return string(marshal)
}

// podFilterFuncreturns true if pod shpuld be mutated
type podFilterFunc func(pod *corev1.Pod) bool

// podSpecMutateFunc mutates the pod spec-level fields of the pod, excluding containers
type podSpecMutateFunc func(pod *corev1.Pod, ps *patchSet) error

// containerMutateFunc mutates the container-level fields of the pod
type containerMutateFunc func(path string, container *corev1.Container, ps *patchSet) error

// podMutatorFunc combines pod-spec and container mutations
type podMutatorFunc func(pod *corev1.Pod, ps *patchSet) error

func podMutator(podM podSpecMutateFunc, containerM containerMutateFunc) podMutatorFunc {
	return func(pod *corev1.Pod, ps *patchSet) error {
		// First patch: pod level securityPolicy
		if err := podM(pod, ps); err != nil {
			return nil
		}
		// Next: patch containers
		mutateContainers := func(path string, containers []corev1.Container) error {
			if len(containers) > 0 {
				for idx, ctx := range containers {
					newPath := fmt.Sprintf("%s/%d", path, idx)
					if err := containerM(newPath, &ctx, ps); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := mutateContainers("/spec/initContainers", pod.Spec.InitContainers); err != nil {
			return err
		}
		if err := mutateContainers("/spec/containers", pod.Spec.Containers); err != nil {
			return err
		}
		return nil
	}
}

// podAdmission analizes admission request and mutates it
func podAdmission(ar v1.AdmissionReview, codecs *serializer.CodecFactory, filter podFilterFunc, mutator podMutatorFunc) *v1.AdmissionResponse {
	klog.V(2).Info("mutating pods")
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		klog.Errorf("expect resource to be %s", podResource)
		return nil
	}

	raw := ar.Request.Object.Raw
	pod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		klog.Error(err)
		return webhook.V1AdmissionError(err)
	}
	reviewResponse := v1.AdmissionResponse{}
	reviewResponse.Allowed = true

	ps := &patchSet{
		patches: make([]jsonPatch, 0, 16),
	}
	if filter(&pod) {
		if err := mutator(&pod, ps); err != nil {
			klog.Error(err)
		} else {
			patchBytes, err := ps.Json()
			if err != nil {
				klog.Error(err)
			} else {
				if len(patchBytes) > 0 {
					reviewResponse.Patch = patchBytes
					pt := v1.PatchTypeJSONPatch
					reviewResponse.PatchType = &pt
				}
			}
		}
	}
	return &reviewResponse
}

// shouldMutateSecurityContext returns true if the pod must be mutated
func shouldMutateSecurityContext(pod *corev1.Pod) bool {
	labels := pod.GetObjectMeta().GetLabels()
	// skip pods labeled with priviledged enforcement
	if priv, ok := labels["pod-security.kubernetes.io/enforce"]; ok && priv == "privileged" {
		return false
	}
	return true
}

// mutatePodSecurityContext mutates the pod with the required securityContext configs
func mutatePodSecurityContext(pod *corev1.Pod, ps *patchSet) error {
	sc := pod.Spec.SecurityContext
	op := "replace"
	modified := false
	if sc == nil {
		sc = &corev1.PodSecurityContext{}
		op = "add"
	}
	if sc.RunAsUser == nil {
		var uid int64 = InjectedUID
		sc.RunAsUser = &uid
		modified = true
	}
	if sc.RunAsGroup == nil {
		var gid int64 = *sc.RunAsUser
		sc.RunAsGroup = &gid
		modified = true
	}
	if sc.RunAsNonRoot == nil {
		var nonRoot = (*sc.RunAsUser != 0)
		sc.RunAsNonRoot = &nonRoot
		modified = true
	}
	if sc.FSGroup == nil {
		var gid int64 = *sc.RunAsUser
		sc.FSGroup = &gid
		modified = true
	}
	if sc.SeccompProfile == nil {
		sc.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
		modified = true
	}
	if !modified {
		return nil
	}
	return ps.append(op, "/spec/securityContext", sc)
}

func mutateContainerSecurityContext(path string, container *corev1.Container, ps *patchSet) error {
	sc := container.SecurityContext
	op := "replace"
	modified := false
	if sc == nil {
		sc = &corev1.SecurityContext{}
		op = "add"
	}
	if sc.AllowPrivilegeEscalation == nil {
		modified = true
		var ape = false
		sc.AllowPrivilegeEscalation = &ape
	}
	if sc.Capabilities == nil {
		modified = true
		sc.Capabilities = &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
	}
	if !modified {
		return nil
	}
	return ps.append(op, fmt.Sprintf("%s/securityContext", path), sc)
}

func mutateSecurityContext(ar v1.AdmissionReview, codecs *serializer.CodecFactory) *v1.AdmissionResponse {
	return podAdmission(ar, codecs, shouldMutateSecurityContext, podMutator(mutatePodSecurityContext, mutateContainerSecurityContext))
}
