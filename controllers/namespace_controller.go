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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=namespaces/finalizers,verbs=update

const (
	previousSchedulerName = "k8s-pause/previousScheduler"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
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
	if suspended, ok := ns.Annotations[suspendedAnnotation]; ok {
		if suspended == "true" {
			suspend = true
		}
	}

	var res ctrl.Result

	if suspend == true {
		logger.Info("make sure namespace is suspended")
		res, err = r.suspend(ctx, ns, logger)
	} else {
		logger.Info("make sure namespace is resumed")
		res, err = r.resume(ctx, ns, logger)
	}

	return res, err
}

func (r *NamespaceReconciler) resume(ctx context.Context, ns corev1.Namespace, logger logr.Logger) (ctrl.Result, error) {
	var list corev1.PodList
	if err := r.List(ctx, &list, client.InNamespace(ns.Name)); err != nil {
		return ctrl.Result{}, err
	}

	for _, pod := range list.Items {
		if pod.Status.Phase == phaseSuspended && pod.Spec.SchedulerName == schedulerName {

			if len(pod.ObjectMeta.OwnerReferences) > 0 {
				err := r.Client.Delete(ctx, &pod)
				if err != nil {
					logger.Error(err, "failed to delete pod while resuming", "pod", pod.Name)
				}
			} else {
				clone := pod.DeepCopy()
				if scheduler, ok := clone.Annotations[previousSchedulerName]; ok {
					clone.Spec.SchedulerName = scheduler
					delete(clone.Annotations, previousSchedulerName)
				} else {
					clone.Spec.SchedulerName = ""
				}

				err := r.Client.Delete(ctx, &pod)
				if err != nil {
					logger.Error(err, "failed to delete pod while resuming", "pod", pod.Name)
					continue
				}

				err = r.Client.Create(ctx, clone)
				if err != nil {
					logger.Error(err, "failed to recreate pod with previous scheduler while resuming", "pod", pod.Name)
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *NamespaceReconciler) suspend(ctx context.Context, ns corev1.Namespace, logger logr.Logger) (ctrl.Result, error) {
	var list corev1.PodList
	if err := r.List(ctx, &list, client.InNamespace(ns.Name)); err != nil {
		return ctrl.Result{}, err
	}

	for _, pod := range list.Items {
		if pod.Spec.SchedulerName != schedulerName {
			// We assume the pod is managed by another controller if there is an existing owner ref
			if len(pod.ObjectMeta.OwnerReferences) > 0 {
				err := r.Client.Delete(ctx, &pod)
				if err != nil {
					logger.Error(err, "failed to delete pod while suspending", "pod", pod.Name)
				}

				// If there is no owner lets clone the pod and swap the scheduler
			} else {
				clone := pod.DeepCopy()
				clone.Spec.SchedulerName = schedulerName

				if clone.Annotations == nil {
					clone.Annotations = make(map[string]string)
				}

				clone.Annotations[previousSchedulerName] = pod.Spec.SchedulerName

				err := r.Client.Delete(ctx, &pod)
				if err != nil {
					logger.Error(err, "failed to delete pod while suspending", "pod", pod.Name)
					continue
				}

				err = r.Client.Create(ctx, clone)
				if err != nil {
					logger.Error(err, "failed to recreate pod with k8s-pause scheduler while suspending", "pod", pod.Name)
				}
			}
		}
	}

	return ctrl.Result{}, nil
}
