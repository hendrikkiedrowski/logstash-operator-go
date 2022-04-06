/*
Copyright 2021.

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
	"reflect"
	"time"

	logstashv1alpha1 "github.com/hendrikkiedrowski/logstash-operator-go/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// LogstashReconciler reconciles a Logstash object
type LogstashReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *LogstashReconciler) fetchLogstash(ctx context.Context, namespace types.NamespacedName) (*logstashv1alpha1.Logstash, *ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	logstash := new(logstashv1alpha1.Logstash)
	err := r.Get(ctx, namespace, logstash)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("Logstash resource not found. Ignoring since object must be deleted")
			return nil, &ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Logstash - requeuing")
		return nil, &ctrl.Result{}, err
	}
	return logstash, nil, nil
}

func (r *LogstashReconciler) fetchOrCreateStatefulSet(ctx context.Context, logstash *logstashv1alpha1.Logstash) (*appsv1.StatefulSet, *ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	namespace := types.NamespacedName{Name: logstash.Name, Namespace: logstash.Namespace}

	var foundSfs *appsv1.StatefulSet
	err := r.Get(ctx, namespace, foundSfs)

	if err != nil && k8serrors.IsNotFound(err) {
		// Define a new stateful set
		newSfs := r.statefulsetForLogstash(logstash)
		log.Info("Creating a new Stateful Set", "StatefulSet.Namespace", newSfs.Namespace, "StatefulSet.Name", newSfs.Name)
		err = r.Create(ctx, newSfs)
		if err != nil {
			log.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", newSfs.Namespace, "StatefulSet.Name", newSfs.Name)
			return nil, &ctrl.Result{}, err
		}
		// Stateful Set created successfully - return and requeue
		return nil, &ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Stateful Set")
		return nil, &ctrl.Result{}, err

	}
	return foundSfs, nil, nil
}

func (r *LogstashReconciler) updateStatefulSet(ctx context.Context, logstash *logstashv1alpha1.Logstash, sfs *appsv1.StatefulSet) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	replicaCount := logstash.Spec.ReplicaCount
	if *sfs.Spec.Replicas != replicaCount {
		sfs.Spec.Replicas = &replicaCount
		err := r.Update(ctx, sfs)
		if err != nil {
			log.Error(err, "Failed to update Stateful Set", "StatefulSet.Namespace", sfs.Namespace, "StatefulSet.Name", sfs.Name)
			return &ctrl.Result{}, err
		}
		// Ask to requeue after 1 minute in order to give enough time for the
		// pods be created on the cluster side and the operand be able
		// to do the next update step accurately.
		return &ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	return nil, nil
}

func (r *LogstashReconciler) findLogstashStateChanges(ctx context.Context, logstash *logstashv1alpha1.Logstash) (bool, *ctrl.Result, error) {
	// Update the Logstash status with the pod names
	// List the pods for this logstash's stateful set
	log := ctrllog.FromContext(ctx)

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(logstash.Namespace),
		client.MatchingLabels(labelsForLogstash(logstash.Name)),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Logstash.Namespace", logstash.Namespace, "Logstash.Name", logstash.Name)
		return false, &ctrl.Result{}, err
	}

	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, logstash.Status.Nodes) {
		logstash.Status.Nodes = podNames
		return true, nil, nil
	}
	return false, nil, nil
}

func (r *LogstashReconciler) updateLogstashState(ctx context.Context, logstash *logstashv1alpha1.Logstash) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	err := r.Status().Update(ctx, logstash)
	if err != nil {
		log.Error(err, "Failed to update Logstash status")
		return &ctrl.Result{}, err
	}
	return nil, nil
}

//+kubebuilder:rbac:groups=logstash.vkiedrowski.de,resources=logstashes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=logstash.vkiedrowski.de,resources=logstashes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=logstash.vkiedrowski.de,resources=logstashes/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Logstash object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *LogstashReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logstash, returnResult, err := r.fetchLogstash(ctx, req.NamespacedName)
	if returnResult != nil {
		return *returnResult, err
	}

	sfs, returnResult, err := r.fetchOrCreateStatefulSet(ctx, logstash)
	if returnResult != nil {
		return *returnResult, err
	}

	returnResult, err = r.updateStatefulSet(ctx, logstash, sfs)
	if returnResult != nil {
		return *returnResult, err
	}

	hasChanged, returnResult, err := r.findLogstashStateChanges(ctx, logstash)
	if returnResult != nil {
		return *returnResult, err
	}

	if hasChanged {
		returnResult, err = r.updateLogstashState(ctx, logstash)
		if returnResult != nil {
			return *returnResult, err
		}
	}

	return ctrl.Result{}, nil
}

// statefulsetForLogstash returns a logstash Statefulset object
func (r *LogstashReconciler) statefulsetForLogstash(m *logstashv1alpha1.Logstash) *appsv1.StatefulSet {
	ls := labelsForLogstash(m.Name)
	replicas := m.Spec.ReplicaCount
	resources := make(corev1.ResourceList)
	resources[corev1.ResourceStorage] = m.Spec.Storage.Size
	pvcs := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-data", m.Name),
				Namespace: m.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      m.Spec.Storage.AccessModes,
				StorageClassName: &m.Spec.Storage.StorageClassName,
				Resources: corev1.ResourceRequirements{
					Requests: resources,
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{},
		},
	}

	sfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "logstash:7.14.2",
						Name:  "logstash",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 9600,
							Name:          "logstash",
						}},
					}},
				},
			},
			VolumeClaimTemplates: pvcs,
			ServiceName:          fmt.Sprintf("%s-headless", m.Name),
		},
	}
	// Set Logstash instance as the owner and controller
	ctrl.SetControllerReference(m, sfs, r.Scheme)
	return sfs
}

// labelsForLogstash returns the labels for selecting the resources
// belonging to the given logstash CR name.
func labelsForLogstash(name string) map[string]string {
	return map[string]string{"app": "logstash", "logstash_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogstashReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&logstashv1alpha1.Logstash{}).
		Complete(r)
}
