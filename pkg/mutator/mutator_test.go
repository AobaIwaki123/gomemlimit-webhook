package mutator

import (
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestMutator_Mutate(t *testing.T) {
	m := NewMutator()

	tests := []struct {
		name          string
		pod           corev1.Pod
		wantPatched   bool
		expectedEnvs  map[string]string // container 0 expected envs
		expectedCount int
	}{
		{
			name: "No annotation - skip mutation",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
							},
						},
					},
				},
			},
			wantPatched: false,
		},
		{
			name: "Annotation enabled - calculate 90% GOMEMLIMIT",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						AnnotationInject: "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
							},
						},
					},
				},
			},
			wantPatched: true,
			expectedEnvs: map[string]string{
				"GOMEMLIMIT": "450MiB",
			},
			expectedCount: 1,
		},
		{
			name: "Custom ratio 80% and custom GOGC",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						AnnotationInject: "true",
						AnnotationRatio:  "0.80",
						AnnotationGOGC:   "200",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
			wantPatched: true,
			expectedEnvs: map[string]string{
				"GOMEMLIMIT": "819MiB", // 1024 * 0.8 = 819.2 -> 819MiB or bytes
				"GOGC":       "200",
			},
			expectedCount: 1, // 1 patch operation with slice of 2 envs (because container.Env was empty)
		},
		{
			name: "Existing GOMEMLIMIT should not be overwritten",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						AnnotationInject: "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Env: []corev1.EnvVar{
								{Name: "GOMEMLIMIT", Value: "100MiB"},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
							},
						},
					},
				},
			},
			wantPatched: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			podRaw, err := json.Marshal(tc.pod)
			if err != nil {
				t.Fatalf("failed to marshal pod: %v", err)
			}

			review := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					UID: "12345",
					Kind: metav1.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Pod",
					},
					Object: runtime.RawExtension{
						Raw: podRaw,
					},
				},
			}

			res := m.Mutate(review)
			if !res.Allowed {
				t.Fatalf("expected allowed=true, got false: %v", res.Result)
			}

			if tc.wantPatched {
				if len(res.Patch) == 0 {
					t.Fatalf("expected patch, but got none")
				}
				var patches []PatchOperation
				if err := json.Unmarshal(res.Patch, &patches); err != nil {
					t.Fatalf("failed to unmarshal patch: %v", err)
				}
				if len(patches) != tc.expectedCount {
					t.Errorf("expected %d patch ops, got %d (%+v)", tc.expectedCount, len(patches), patches)
				}
			} else {
				if len(res.Patch) > 0 {
					t.Fatalf("expected no patch, but got: %s", string(res.Patch))
				}
			}
		})
	}
}
