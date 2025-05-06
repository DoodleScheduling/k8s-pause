/*
Copyright 2022 Doodle.

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

package controllers

import (
	"context"

	"github.com/doodlescheduling/k8s-pause/pkg/common"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

const (
	phaseSuspended = "Suspended"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	Client client.WithWatch
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type PodReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager, opts PodReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("Namespace", req.Namespace, "Name", req.NamespacedName)

	// Fetch the pod
	pod := corev1.Pod{}

	err := r.Client.Get(ctx, req.NamespacedName, &pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	var suspend bool
	if suspended, ok := pod.Annotations[SuspendedAnnotation]; ok {
		if suspended == "true" {
			suspend = true
		}
	}

	if suspend {
		logger.Info("make sure pod is suspended")
		if ignore, ok := pod.Annotations[common.IgnoreAnnotation]; ok && ignore == "true" {
			return ctrl.Result{}, nil
		} else {
			if err := common.SuspendPod(ctx, r.Client, pod, logger); err != nil {
				logger.Error(err, "failed to suspend pod", "pod", pod.Name)
			}
		}

	} else {
		logger.Info("make sure pod is resumed")
		if ignore, ok := pod.Annotations[common.IgnoreAnnotation]; ok && ignore == "true" {
			return ctrl.Result{}, nil
		} else {
			if pod.Status.Phase == phaseSuspended && pod.Spec.SchedulerName == SchedulerName {
				if len(pod.ObjectMeta.OwnerReferences) > 0 {
					err := r.Client.Delete(ctx, &pod)
					if err != nil {
						logger.Error(err, "failed to delete pod while resuming", "pod", pod.Name)
					}
				} else {
					clone := pod.DeepCopy()

					// We won't be able to create the object with the same resource version
					clone.ObjectMeta.ResourceVersion = ""

					// Remove assigned node to avoid scheduling
					clone.Spec.NodeName = ""

					// Reset status, not needed as its ignored but nice
					clone.Status = corev1.PodStatus{}

					if scheduler, ok := clone.Annotations[common.PreviousSchedulerName]; ok {
						clone.Spec.SchedulerName = scheduler
						delete(clone.Annotations, common.PreviousSchedulerName)
					} else {
						clone.Spec.SchedulerName = ""
					}

					err := common.RecreatePod(ctx, r.Client, pod, clone)
					if err != nil {
						logger.Error(err, "recrete unowned pod failed", "pod", pod.Name)
					}
				}
			}
		}

	}

	if pod.Spec.SchedulerName == SchedulerName {
		pod.Status.Phase = phaseSuspended

		// Update status after reconciliation.
		if err = r.patchStatus(ctx, &pod); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *PodReconciler) patchStatus(ctx context.Context, pod *corev1.Pod) error {
	key := client.ObjectKeyFromObject(pod)
	latest := &corev1.Pod{}
	if err := r.Client.Get(ctx, key, latest); err != nil {
		return err
	}

	return r.Client.Status().Patch(ctx, pod, client.MergeFrom(latest))
}
