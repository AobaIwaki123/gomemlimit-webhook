package mutator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AnnotationInject = "gomemlimit.aooba.net/inject"
	AnnotationRatio  = "gomemlimit.aooba.net/ratio"
	AnnotationGOGC   = "gomemlimit.aooba.net/gogc"

	DefaultRatio = 0.90
)

// PatchOperation represents an RFC 6902 JSON patch operation.
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// Mutator inspects Pod specifications and generates mutation patches.
type Mutator struct{}

// NewMutator creates a new Mutator instance.
func NewMutator() *Mutator {
	return &Mutator{}
}

// Mutate processes an AdmissionReview request and returns the AdmissionResponse.
func (m *Mutator) Mutate(review *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := review.Request
	if req == nil {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "AdmissionReview request is nil",
			},
		}
	}

	res := &admissionv1.AdmissionResponse{
		UID:     req.UID,
		Allowed: true,
	}

	// Only mutate Pod resources
	if req.Kind.Kind != "Pod" {
		return res
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		res.Allowed = false
		res.Result = &metav1.Status{
			Message: fmt.Sprintf("failed to unmarshal Pod: %v", err),
		}
		return res
	}

	patches, err := m.MutatePod(&pod)
	if err != nil {
		res.Allowed = false
		res.Result = &metav1.Status{
			Message: fmt.Sprintf("failed to generate patches: %v", err),
		}
		return res
	}

	if len(patches) > 0 {
		patchBytes, err := json.Marshal(patches)
		if err != nil {
			res.Allowed = false
			res.Result = &metav1.Status{
				Message: fmt.Sprintf("failed to marshal JSON patch: %v", err),
			}
			return res
		}
		patchType := admissionv1.PatchTypeJSONPatch
		res.Patch = patchBytes
		res.PatchType = &patchType
	}

	return res
}

// MutatePod determines if the Pod is eligible and generates patches.
func (m *Mutator) MutatePod(pod *corev1.Pod) ([]PatchOperation, error) {
	annotations := pod.Annotations
	if annotations == nil {
		return nil, nil
	}

	injectVal, ok := annotations[AnnotationInject]
	if !ok || strings.ToLower(injectVal) != "true" {
		return nil, nil
	}

	ratio := DefaultRatio
	if ratioStr, ok := annotations[AnnotationRatio]; ok {
		parsed, err := strconv.ParseFloat(ratioStr, 64)
		if err == nil && parsed > 0.1 && parsed <= 1.0 {
			ratio = parsed
		}
	}

	var gogcVal string
	if v, ok := annotations[AnnotationGOGC]; ok && v != "" {
		gogcVal = v
	}

	var patches []PatchOperation

	for i, container := range pod.Spec.Containers {
		containerPatches := m.mutateContainer(i, container, ratio, gogcVal)
		patches = append(patches, containerPatches...)
	}

	return patches, nil
}

func (m *Mutator) mutateContainer(index int, container corev1.Container, ratio float64, gogcVal string) []PatchOperation {
	// Skip if GOMEMLIMIT is already configured
	hasMemlimit := false
	hasGOGC := false
	for _, env := range container.Env {
		if env.Name == "GOMEMLIMIT" {
			hasMemlimit = true
		}
		if env.Name == "GOGC" {
			hasGOGC = true
		}
	}

	var newEnvs []corev1.EnvVar

	// Calculate GOMEMLIMIT if not explicitly defined
	if !hasMemlimit {
		var memLimitBytes int64
		if limit, ok := container.Resources.Limits[corev1.ResourceMemory]; ok && !limit.IsZero() {
			memLimitBytes = limit.Value()
		} else if req, ok := container.Resources.Requests[corev1.ResourceMemory]; ok && !req.IsZero() {
			// Fallback to memory request if limits are not set
			memLimitBytes = req.Value()
		}

		if memLimitBytes > 0 {
			targetBytes := int64(float64(memLimitBytes) * ratio)
			// Format readable units (e.g., MiB if aligned, or bytes if exact)
			var memlimitStr string
			if targetBytes%(1024*1024) == 0 {
				memlimitStr = fmt.Sprintf("%dMiB", targetBytes/(1024*1024))
			} else {
				memlimitStr = fmt.Sprintf("%dB", targetBytes)
			}

			newEnvs = append(newEnvs, corev1.EnvVar{
				Name:  "GOMEMLIMIT",
				Value: memlimitStr,
			})
		}
	}

	// Inject GOGC if specified in annotation and not set
	if gogcVal != "" && !hasGOGC {
		newEnvs = append(newEnvs, corev1.EnvVar{
			Name:  "GOGC",
			Value: gogcVal,
		})
	}

	if len(newEnvs) == 0 {
		return nil
	}

	var patches []PatchOperation
	basePath := fmt.Sprintf("/spec/containers/%d/env", index)

	if len(container.Env) == 0 {
		// Replace or add entire env list
		patches = append(patches, PatchOperation{
			Op:    "add",
			Path:  basePath,
			Value: newEnvs,
		})
	} else {
		// Append each new env var to existing list
		for _, env := range newEnvs {
			patches = append(patches, PatchOperation{
				Op:    "add",
				Path:  basePath + "/-",
				Value: env,
			})
		}
	}

	return patches
}
