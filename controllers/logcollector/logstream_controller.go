/*
Copyright 2022.

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

package logcollector

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source" // Required for Watching

	logcollectorv1alpha1 "github.com/fanfengqiang/log-collector-operator/apis/logcollector/v1alpha1"
)

// LogStreamReconciler reconciles a LogStream object
type LogStreamReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=logcollector.iffq.top,resources=logstreams,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=logcollector.iffq.top,resources=logstreams/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=logcollector.iffq.top,resources=logstreams/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LogStream object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *LogStreamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var logStream logcollectorv1alpha1.LogStream
	if err := r.Get(ctx, req.NamespacedName, &logStream); err != nil {
		log.Error(err, "unable to fetch LogStream")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	labelSelector, err := metav1.LabelSelectorAsSelector(&logStream.Spec.PodSelector)
	if err != nil {
		log.Error(err, "labelSelector err")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace), &client.ListOptions{
		LabelSelector: labelSelector,
	}); err != nil {
		log.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}
	if len(pods.Items) < 1 {
		log.Error(err, "pod num lt 1")
		return ctrl.Result{}, err
	}
	// TODO(user): your logic here
	log.Info(fmt.Sprintf("namespace:%s, pod:%s,pod num:%d", pods.Items[0].Namespace, pods.Items[0].Name, len(pods.Items)))
	return ctrl.Result{Requeue: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogStreamReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&logcollectorv1alpha1.LogStream{}).
		Owns(&logcollectorv1alpha1.LogGathererConf{}).
		Watches(
			&source.Kind{Type: &corev1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(r.findPodForLogStream),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		WithEventFilter(predicate.LabelChangedPredicate{}).
		WithEventFilter(predicate.AnnotationChangedPredicate{}).
		Complete(r)
}

func (r *LogStreamReconciler) findPodForLogStream(pod client.Object) []reconcile.Request {
	allLogStreams := &logcollectorv1alpha1.LogStreamList{}
	listOps := &client.ListOptions{
		// FieldSelector: fields.OneTermEqualSelector(configMapField, configMap.GetName()),
		Namespace: pod.GetNamespace(),
	}
	err := r.List(context.TODO(), allLogStreams, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := []reconcile.Request{}

	for _, item := range allLogStreams.Items {
		labelSelector, _ := metav1.LabelSelectorAsSelector(&item.Spec.PodSelector)
		// fmt.Println(labelSelector)
		if labelSelector.Matches(labels.Set(pod.GetLabels())) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			})
		}

	}
	return requests
}
