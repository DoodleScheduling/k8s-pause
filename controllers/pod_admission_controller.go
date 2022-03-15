package controllers

import (
	"context"
	"encoding/json"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=k8s-pause.infra.doodle.com,admissionReviewVersions=v1,sideEffects=None

const (
	suspendedAnnotation = "k8s-pause/suspend"
	schedulerName       = "k8s-pause"
)

// podAnnotator annotates Pods
type Scheduler struct {
	Client  client.Client
	decoder *admission.Decoder
}

// podAnnotator adds an annotation to every incoming pods.
func (a *Scheduler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := a.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var ns corev1.Namespace
	err = a.Client.Get(ctx, types.NamespacedName{
		Name: req.Namespace,
	}, &ns)

	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var suspend bool
	if suspended, ok := ns.Annotations[suspendedAnnotation]; ok {
		if suspended == "true" {
			suspend = true
		}
	}

	if suspend == false {
		return admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: true,
			},
		}
	}

	pod.Spec.SchedulerName = schedulerName

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectDecoder injects the decoder.
func (a *Scheduler) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
