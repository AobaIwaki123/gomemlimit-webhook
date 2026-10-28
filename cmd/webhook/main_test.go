package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/AobaIwaki123/gomemlimit-webhook/pkg/mutator"
)

func TestMutateEndpoint_Integration(t *testing.T) {
	m := mutator.NewMutator()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var review admissionv1.AdmissionReview
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := m.Mutate(&review)
		respReview := admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			},
			Response: response,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(respReview)
	})

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample-app",
			Annotations: map[string]string{
				mutator.AnnotationInject: "true",
				mutator.AnnotationRatio:  "0.85",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "web",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},
				},
			},
		},
	}

	podRaw, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("failed to marshal pod: %v", err)
	}

	reqReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid-456",
			Kind: metav1.GroupVersionKind{
				Version: "v1",
				Kind:    "Pod",
			},
			Object: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	payload, err := json.Marshal(reqReview)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", res.StatusCode)
	}

	var respReview admissionv1.AdmissionReview
	if err := json.NewDecoder(res.Body).Decode(&respReview); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respReview.Response == nil {
		t.Fatalf("expected non-nil response")
	}
	if !respReview.Response.Allowed {
		t.Fatalf("expected allowed=true, got false")
	}
	if len(respReview.Response.Patch) == 0 {
		t.Fatalf("expected patch in response")
	}

	var patches []mutator.PatchOperation
	if err := json.Unmarshal(respReview.Response.Patch, &patches); err != nil {
		t.Fatalf("failed to parse patch: %v", err)
	}

	// 200Mi * 0.85 = 170MiB
	t.Logf("Generated Patch: %+v", patches)
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch op, got %d", len(patches))
	}
}
