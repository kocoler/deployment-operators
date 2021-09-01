/*


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
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sync"

	appsv1alpha1 "github.com/kocoler/deployment-operators/api/v1alpha1"
)

// DeploymentOperatorsReconciler reconciles a DeploymentOperators object
type DeploymentOperatorsReconciler struct {
	client.Client
	Log         logr.Logger
	Scheme      *runtime.Scheme
	Deployments sync.Map
	Replicas    sync.Map
	// before: just for those rs event comes before their deployment create event
	// now: all pending task
	PendingTask           sync.Map
	Count                 int
	MessageSenderInstance *MessageSender
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &DeploymentOperatorsReconciler{}

type Deployments struct {
	Name           string // deployment name
	NowVersion     string // now record id
	TargetVersion  string // target record id
	TargetReplicas int32  // target replica count
}

type Replicas struct {
	Deployment string // owener deployment name
	Name       string
	Version    string // record id
	Status     int
	Cause      string // explain now status
}

// +kubebuilder:rbac:groups=apps.my.domain,resources=deploymentoperators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.my.domain,resources=deploymentoperators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=appsv1,resources=replicasets,verbs=get;update;patch;delete;list;watch
// +kubebuilder:rbac:groups=appsv1,resources=deployments,verbs=get;update;patch;delete;list;watch

func (r *DeploymentOperatorsReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("deploymentoperators", req.NamespacedName)

	var err error

	//fetch the deployment operator instance
	instance := &appsv1alpha1.DeploymentOperators{}
	err = r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Request deployment operator instance failed.")
			return reconcile.Result{}, err
		}
	} else {
		if instance.DeletionTimestamp != nil {
			return reconcile.Result{}, err
		}

		fmt.Println("operator instance !!!!", instance)

		r.MessageSenderInstance.Hosts = instance.Spec.CustomerEndpoints
		r.MessageSenderInstance.Initial = true
		for _, v := range r.MessageSenderInstance.MessagesBuffer {
			r.MessageSenderInstance.SendMessage(v)
		}
	}

	deploy := &appsv1.Deployment{}
	err = r.Client.Get(context.TODO(), req.NamespacedName, deploy)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "get deployment error")
			return reconcile.Result{}, nil
		}
	} else {
		fmt.Println("name", deploy.Name)
		// add to map
		// two status: deployment exists -> continue ... deployment not exists -> update
		// every goroutine handles different event -> needn't consider concurrent for the same key
		value, ok := r.Deployments.Load(deploy.Name)
		if ok {
			preDeploy := value.(*Deployments)
			preDeploy.TargetVersion = deploy.Labels[RecordID]
			preDeploy.TargetReplicas = *deploy.Spec.Replicas
			r.Deployments.Store(deploy.Name, preDeploy)
		} else {
			deployment := &Deployments{
				Name:           deploy.Name,
				NowVersion:     "",
				TargetVersion:  deploy.Labels[RecordID],
				TargetReplicas: *deploy.Spec.Replicas,
			}
			r.Deployments.Store(deploy.Name, deployment)
		}

		return reconcile.Result{}, err
	}

	rs := &appsv1.ReplicaSet{}
	err = r.Client.Get(context.TODO(), req.NamespacedName, rs)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "get replicaset error")
			return reconcile.Result{}, nil
		}
	} else {
		fmt.Println("rs name:, rs status:", rs.Name, rs.Status, *rs.Spec.Replicas)

		var deployName string
		for _, v := range rs.OwnerReferences {
			if v.Kind == "Deployment" {
				deployName = v.Name
			}
		}

		if *rs.Spec.Replicas == 0 {
			// delete useless rs
			err = r.Delete(context.TODO(), rs)
			if err != nil {
				return reconcile.Result{}, err
			}
			// will be replaced, ignore
			return reconcile.Result{}, nil
		}

		// success
		if rs.Status.AvailableReplicas >= rs.Status.FullyLabeledReplicas && rs.Status.FullyLabeledReplicas != 0 {
			replicaset := &Replicas{
				Deployment: deployName,
				Name:       rs.Name,
				Version:    rs.Labels[RecordID],
				Status:     ReplicaStatusSuccess,
				Cause:      "", // TODO
			}
			r.Replicas.Store(rs.Name, replicaset)
			r.PendingTask.Store(rs.Name, replicaset)
		} else if rs.Status.FullyLabeledReplicas != 0 {
			// unhealthy
			replicaset := &Replicas{
				Deployment: deployName,
				Name:       rs.Name,
				Version:    rs.Labels[RecordID],
				Status:     ReplicaStatusPending,
				Cause:      "", // TODO
			}
			r.Replicas.Store(rs.Name, replicaset)
			r.PendingTask.Store(rs.Name, replicaset)
		}
	}

	// handle task
	r.PendingTask.Range(func(key, value interface{}) bool {
		rs := value.(*Replicas)
		v, ok := r.Deployments.Load(rs.Deployment)
		if ok {
			deploy := v.(*Deployments)
			if deploy.NowVersion == rs.Version {
				if rs.Status != ReplicaStatusSuccess {
					// failed / pending
					// ori was healthy but rs failed suddenly -> pending
					_ = r.MessageSenderInstance.SendMessage(Message{
						Version: rs.Version,
						Status:  ReplicaStatusPending,
						Cause:   "deploy failed",
					})
				}
			} else if deploy.TargetVersion == rs.Version {
				if rs.Status == ReplicaStatusSuccess {
					deploy.NowVersion = rs.Version

					// send message
					_ = r.MessageSenderInstance.SendMessage(Message{
						Version: rs.Version,
						Status:  ReplicaStatusSuccess,
						Cause:   "deploy success",
					})
					// TODO
				} else {
					// send message
					_ = r.MessageSenderInstance.SendMessage(Message{
						Version: rs.Version,
						Status:  ReplicaStatusPending,
						Cause:   "deploying",
					})
					// TODO
				}
			} else {
				_ = r.MessageSenderInstance.SendMessage(Message{
					Version: rs.Version,
					Status:  ReplicaStatusSuccess,
					Cause:   "deploy success",
				})
			}
		}

		r.PendingTask.Delete(key)

		return true
	})

	return reconcile.Result{}, nil
}

type ResourceChangedPredicate struct {
	predicate.Funcs
}

func compareMaps(m1, m2 map[string]string) bool {
	if len(m1) != len(m2) {
		return false
	}

	for k, v := range m1 {
		if value, ok := m2[k]; ok {
			if v != value {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

func (r *ResourceChangedPredicate) Update(e event.UpdateEvent) bool {
	oldObj, ok1 := e.ObjectOld.(*appsv1.ReplicaSet)
	newObj, ok2 := e.ObjectNew.(*appsv1.ReplicaSet)

	if ok1 && ok2 {
		oldObjStatus := oldObj.Status
		newObjStatus := newObj.Status

		// replicaset need to compare replica num
		if oldObjStatus.AvailableReplicas == newObjStatus.AvailableReplicas && oldObjStatus.ReadyReplicas == newObjStatus.ReadyReplicas && oldObjStatus.Replicas == newObjStatus.Replicas {
			return false
		}

		return true
	}

	// deployment need to compare labels
	if !compareMaps(e.MetaOld.GetLabels(), e.MetaNew.GetLabels()) {
		return true
	}

	return false
}

type ResourceLabelPredicate struct {
	predicate.Funcs
}

func judgeResourceLabel(labels map[string]string) bool {
	//if _, ok := labels[RecordID]; !ok {
	//	return false
	//}

	if _, ok := labels[MKE]; !ok {
		return false
	}

	return true
}

func (r *ResourceLabelPredicate) Update(e event.UpdateEvent) bool {
	labels := e.MetaNew.GetLabels()

	return judgeResourceLabel(labels)
}

func (r *ResourceLabelPredicate) Create(e event.CreateEvent) bool {
	labels := e.Meta.GetLabels()

	return judgeResourceLabel(labels)
}

func (r *ResourceLabelPredicate) Delete(e event.DeleteEvent) bool {
	labels := e.Meta.GetLabels()

	return judgeResourceLabel(labels)
}

func (r *DeploymentOperatorsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log = ctrl.Log.WithName("deploymentoperators")

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.DeploymentOperators{}).
		// add watch resources
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &appsv1.ReplicaSet{}}, &handler.EnqueueRequestForObject{}).
		// filter update
		WithEventFilter(&ResourceChangedPredicate{}).
		// filter label
		WithEventFilter(&ResourceLabelPredicate{}).
		Complete(r)
}
