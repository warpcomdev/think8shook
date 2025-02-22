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
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/warpcomdev/think8shook/internal/webhook"

	v1 "k8s.io/api/admission/v1"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
	// TODO: try this library to see if it generates correct json patch
	// https://github.com/mattbaird/jsonpatch
)

var (
	certFile string
	keyFile  string
	port     int
)

// CmdWebhook is used by agnhost Cobra.
var CmdWebhook = &cobra.Command{
	Use:   "webhook",
	Short: "Starts a HTTP server, useful for testing MutatingAdmissionWebhook and ValidatingAdmissionWebhook",
	Long: `Starts a HTTP server, useful for testing MutatingAdmissionWebhook and ValidatingAdmissionWebhook.
After deploying it to Kubernetes cluster, the Administrator needs to create a ValidatingWebhookConfiguration
in the Kubernetes cluster to register remote webhook admission controllers.`,
	Args: cobra.MaximumNArgs(0),
	Run:  main,
}

func init() {
	var fs flag.FlagSet
	klog.InitFlags(&fs)

	CmdWebhook.PersistentFlags().StringVarP(&certFile, "tls-cert-file", "c", "",
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	CmdWebhook.PersistentFlags().StringVarP(&keyFile, "tls-private-key-file", "k", "",
		"File containing the default x509 private key matching --tls-cert-file.")
	CmdWebhook.PersistentFlags().IntVarP(&port, "port", "p", 8443,
		"Secure port that the webhook listens on")
	CmdWebhook.Flags().AddGoFlagSet(&fs)

	CmdWebhook.MarkPersistentFlagRequired("tls-cert-file")
	CmdWebhook.MarkPersistentFlagRequired("tls-private-key-file")
}

// AdmitHandler exposes the interface to create a dual v1 / v1beta1 handler
type AdmitHandler interface {
	V1(ar v1.AdmissionReview) *v1.AdmissionResponse
	V1beta1(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse
}

// serve handles the http portion of a request prior to handing to an admit
// function
func serve(w http.ResponseWriter, r *http.Request, admit AdmitHandler, codecs *serializer.CodecFactory) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	klog.V(2).Info(fmt.Sprintf("handling request: %s", body))

	deserializer := codecs.UniversalDeserializer()
	obj, gvk, err := deserializer.Decode(body, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	var responseObj runtime.Object
	switch *gvk {
	case v1beta1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*v1beta1.AdmissionReview)
		if !ok {
			klog.Errorf("Expected v1beta1.AdmissionReview but got: %T", obj)
			return
		}
		responseAdmissionReview := &v1beta1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = admit.V1beta1(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview
	case v1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*v1.AdmissionReview)
		if !ok {
			klog.Errorf("Expected v1.AdmissionReview but got: %T", obj)
			return
		}
		responseAdmissionReview := &v1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = admit.V1(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview
	default:
		msg := fmt.Sprintf("Unsupported group version kind: %v", gvk)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	klog.V(2).Info(fmt.Sprintf("sending response: %v", responseObj))
	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		klog.Error(err)
	}
}

func main(cmd *cobra.Command, args []string) {
	config := webhook.Config{
		CertFile: certFile,
		KeyFile:  keyFile,
	}
	//
	// http.HandleFunc("/always-allow-delay-5s", serveAlwaysAllowDelayFiveSeconds)
	// http.HandleFunc("/always-deny", serveAlwaysDeny)
	// http.HandleFunc("/add-label", serveAddLabel)
	// http.HandleFunc("/pods", servePods)
	// http.HandleFunc("/pods/attach", serveAttachingPods)
	// http.HandleFunc("/mutating-pods", serveMutatePods)
	// http.HandleFunc("/mutating-pods-sidecar", serveMutatePodsSidecar)
	// http.HandleFunc("/configmaps", serveConfigmaps)
	// http.HandleFunc("/mutating-configmaps", serveMutateConfigmaps)
	// http.HandleFunc("/custom-resource", serveCustomResource)
	// http.HandleFunc("/mutating-custom-resource", serveMutateCustomResource)
	// http.HandleFunc("/crd", serveCRD)
	http.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) { w.Write([]byte("ok")) })
	server := &http.Server{
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    65535,
		Addr:              fmt.Sprintf(":%d", port),
		TLSConfig:         config.TLS(),
	}
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		panic(err)
	}
}
