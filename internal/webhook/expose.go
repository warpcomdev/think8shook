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

package webhook

import (
	"crypto/tls"

	v1 "k8s.io/api/admission/v1"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	// TODO: try this library to see if it generates correct json patch
	// https://github.com/mattbaird/jsonpatch
)

// Admit exposes the interface to create a dual v1 / v1beta1 handler
type Admit struct {
	admitHandler
}

func (h Admit) V1(ar v1.AdmissionReview) *v1.AdmissionResponse {
	return h.v1(ar)
}

func (h Admit) V1beta1(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	return h.v1beta1(ar)
}

func V1AdmissionError(err error) *v1.AdmissionResponse {
	return toV1AdmissionResponse(err)
}

func NewDelegateToV1AdmitHandler(f func(v1.AdmissionReview) *v1.AdmissionResponse) Admit {
	return Admit{
		admitHandler: newDelegateToV1AdmitHandler(f),
	}
}

func Codecs() *serializer.CodecFactory {
	return &codecs
}

func (config Config) TLS() *tls.Config {
	return configTLS(config)
}
