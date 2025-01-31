package common

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ProfileAnnotation     = "k8s-pause/profile"
	SuspendedAnnotation   = "k8s-pause/suspend"
	SchedulerName         = "k8s-pause"
	PreviousSchedulerName = "k8s-pause/previousScheduler"
	IgnoreAnnotation      = "k8s-pause/ignore"
)

func SuspendPod(ctx context.Context, client client.WithWatch, pod corev1.Pod, logger logr.Logger) error {
	if pod.Spec.SchedulerName == SchedulerName {
		return nil
	}

	// We assume the pod is managed by another controller if there is an existing owner ref
	if len(pod.ObjectMeta.OwnerReferences) > 0 {
		err := client.Delete(ctx, &pod)
		if err != nil {
			return err
		}

		// If there is no owner lets clone the pod and swap the scheduler
	} else {
		clone := pod.DeepCopy()
		// We won't be able to create the object with the same resource version
		clone.ObjectMeta.ResourceVersion = ""

		// Remove assigned node to avoid scheduling
		clone.Spec.NodeName = ""

		// Reset status, not needed as its ignored but nice
		clone.Status = corev1.PodStatus{}

		// Assign our own scheduler to avoid the default scheduler interfer with the workload
		clone.Spec.SchedulerName = SchedulerName

		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}

		clone.Annotations[PreviousSchedulerName] = pod.Spec.SchedulerName

		err := RecreatePod(ctx, client, pod, clone)
		if err != nil {
			return fmt.Errorf("recrete unowned pod `%s` failed: %w", pod.Name, err)
		}
	}

	return nil
}

func RecreatePod(ctx context.Context, client client.WithWatch, pod corev1.Pod, clone *corev1.Pod) error {
	list := corev1.PodList{}
	watcher, err := client.Watch(ctx, &list)
	if err != nil {
		return fmt.Errorf("failed to start watch stream for pod %s: %w", pod.Name, err)
	}

	ch := watcher.ResultChan()

	err = client.Delete(ctx, &pod)
	if err != nil {
		return fmt.Errorf("failed to delete pod %s: %w", pod.Name, err)
	}

	// Wait for delete event before we can attempt create the clone
	for event := range ch {
		if event.Type == watch.Deleted {
			if val, ok := event.Object.(*corev1.Pod); ok && val.Name == pod.Name {
				err = client.Create(ctx, clone)
				if err != nil {
					return fmt.Errorf("failed to recreate pod %s: %w", pod.Name, err)
				}

				watcher.Stop()
				break
			}
		}
	}

	return nil
}
