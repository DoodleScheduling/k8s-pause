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

	"github.com/doodlescheduling/k8s-pause/api/v1beta1"
	"github.com/doodlescheduling/k8s-pause/pkg/common"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=namespaces/finalizers,verbs=update

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	Client client.WithWatch
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type NamespaceReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager, opts NamespaceReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("Namespace", req.Namespace, "Name", req.NamespacedName)

	// Fetch the ns
	ns := corev1.Namespace{}

	err := r.Client.Get(ctx, req.NamespacedName, &ns)
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
	if suspended, ok := ns.Annotations[SuspendedAnnotation]; ok {
		if suspended == "true" {
			suspend = true
		}
	}

	var profile *v1beta1.ResumeProfile
	if p, ok := ns.Annotations[ProfileAnnotation]; ok {
		profile = &v1beta1.ResumeProfile{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Name:      p,
			Namespace: req.Name,
		}, profile)

		if err != nil {
			return ctrl.Result{}, err
		}
	}

	var res ctrl.Result

	if suspend {
		logger.Info("make sure namespace is suspended")
		res, err = r.suspend(ctx, ns, logger)
	} else {
		logger.Info("make sure namespace is resumed")
		res, err = r.resume(ctx, ns, profile, logger)
		if err != nil {
			return res, err
		}

		// suspend all non matching pods from profile
		if profile != nil {
			return r.suspendNotInProfile(ctx, ns, *profile, logger)
		}
	}

	return res, err
}

func matchesResumeProfile(pod corev1.Pod, profile v1beta1.ResumeProfile) bool {
	for _, match := range profile.Spec.PodSelector {
		selector, err := metav1.LabelSelectorAsSelector(&match)
		if err != nil {
			continue
		}

		if selector.Matches(labels.Set(pod.Labels)) {
			return true
		}
	}

	return false
}

func (r *NamespaceReconciler) resume(ctx context.Context, ns corev1.Namespace, profile *v1beta1.ResumeProfile, logger logr.Logger) (ctrl.Result, error) {
	var list corev1.PodList
	if err := r.Client.List(ctx, &list, client.InNamespace(ns.Name)); err != nil {
		return ctrl.Result{}, err
	}

	for _, pod := range list.Items {
		if ignore, ok := pod.Annotations[common.IgnoreAnnotation]; ok && ignore == "true" {
			continue
		}

		if profile != nil {
			if !matchesResumeProfile(pod, *profile) {
				continue
			}
		}

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

	return ctrl.Result{}, nil
}

func (r *NamespaceReconciler) suspend(ctx context.Context, ns corev1.Namespace, logger logr.Logger) (ctrl.Result, error) {
	var list corev1.PodList
	if err := r.Client.List(ctx, &list, client.InNamespace(ns.Name)); err != nil {
		return ctrl.Result{}, err
	}

	for _, pod := range list.Items {
		if ignore, ok := pod.Annotations[common.IgnoreAnnotation]; ok && ignore == "true" {
			continue
		}

		if err := common.SuspendPod(ctx, r.Client, pod, logger); err != nil {
			logger.Error(err, "failed to suspend pod", "pod", pod.Name)
			continue
		}
	}

	return ctrl.Result{}, nil
}

func (r *NamespaceReconciler) suspendNotInProfile(ctx context.Context, ns corev1.Namespace, profile v1beta1.ResumeProfile, logger logr.Logger) (ctrl.Result, error) {
	var list corev1.PodList
	if err := r.Client.List(ctx, &list, client.InNamespace(ns.Name)); err != nil {
		return ctrl.Result{}, err
	}

	for _, pod := range list.Items {
		if matchesResumeProfile(pod, profile) {
			continue
		}

		if err := common.SuspendPod(ctx, r.Client, pod, logger); err != nil {
			logger.Error(err, "failed to suspend pod", "pod", pod.Name)
			continue
		}
	}

	return ctrl.Result{}, nil
}
