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
	"k8s.io/client-go/util/retry"
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

// +kubebuilder:rbac:groups=logstash.vkiedrowski.de,resources=logstashes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=logstash.vkiedrowski.de,resources=logstashes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=logstash.vkiedrowski.de,resources=logstashes/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=logstash.vkiedrowski.de,resources=logstashpipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=logstash.vkiedrowski.de,resources=logstashpipelines/status,verbs=get;update;patch
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

	if err = r.reconcilePipelineConfigMap(ctx, logstash); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile pipeline ConfigMap: %w", err)
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

func (r *LogstashReconciler) fetchLogstash(ctx context.Context, namespacedName types.NamespacedName) (*logstashv1alpha1.Logstash, *ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("fetching logstash CRD")

	logstash := new(logstashv1alpha1.Logstash)
	err := r.Get(ctx, namespacedName, logstash)
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
	log.Info("fetching or creating stateful set")
	namespace := types.NamespacedName{Name: logstash.Name, Namespace: logstash.Namespace}

	foundSfs := new(appsv1.StatefulSet)
	err := r.Get(ctx, namespace, foundSfs)

	if err != nil && k8serrors.IsNotFound(err) {
		// Define a new stateful set
		newSfs, err := r.statefulsetForLogstash(ctx, logstash)
		if err != nil {
			log.Error(err, "failed to create StatefulSet definition")
			return nil, &ctrl.Result{}, err
		}
		if newSfs == nil {
			log.Error(nil, "StatefulSet definition is nil")
			return nil, &ctrl.Result{}, fmt.Errorf("StatefulSet definition is nil")
		}
		log.Info("creating a new Stateful Set", "StatefulSet.Namespace", newSfs.Namespace, "StatefulSet.Name", newSfs.Name)
		err = r.Create(ctx, newSfs)
		if err != nil {
			log.Error(err, "failed to create new StatefulSet", "StatefulSet.Namespace", newSfs.Namespace, "StatefulSet.Name", newSfs.Name)
			return nil, &ctrl.Result{}, err
		}
		// Stateful Set created successfully - return and requeue
		return nil, &ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "failed to get Stateful Set")
		return nil, &ctrl.Result{}, err
	}
	return foundSfs, nil, nil
}

func (r *LogstashReconciler) updateStatefulSet(ctx context.Context, logstash *logstashv1alpha1.Logstash, sfs *appsv1.StatefulSet) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("updating stateful set")

	updatedSfs, err := r.statefulsetForLogstash(ctx, logstash)
	if err != nil {
		log.Error(err, "Failed to generate updated StatefulSet")
		return &ctrl.Result{}, err
	}

	needsUpdate := false

	// Check replica count
	replicaCount := logstash.Spec.ReplicaCount
	if *sfs.Spec.Replicas != replicaCount {
		needsUpdate = true
		sfs.Spec.Replicas = &replicaCount
	}

	// Compare volumes by name and content instead of using DeepEqual
	volumesChanged := compareVolumes(sfs.Spec.Template.Spec.Volumes, updatedSfs.Spec.Template.Spec.Volumes)
	volumeMountsChanged := compareVolumeMounts(sfs.Spec.Template.Spec.Containers[0].VolumeMounts,
		updatedSfs.Spec.Template.Spec.Containers[0].VolumeMounts)

	if volumesChanged || volumeMountsChanged {
		log.Info("Volumes or volume mounts changed", "volumesChanged", volumesChanged, "volumeMountsChanged", volumeMountsChanged)
		needsUpdate = true
		sfs.Spec.Template.Spec.Volumes = updatedSfs.Spec.Template.Spec.Volumes
		sfs.Spec.Template.Spec.Containers[0].VolumeMounts = updatedSfs.Spec.Template.Spec.Containers[0].VolumeMounts
	}

	if needsUpdate {
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

// Helper function to compare volumes by name and configuration
func compareVolumes(current, desired []corev1.Volume) bool {
	if len(current) != len(desired) {
		return true
	}

	// Create maps for easier comparison by name
	currentVolumes := make(map[string]corev1.Volume)
	for _, v := range current {
		currentVolumes[v.Name] = v
	}

	for _, desiredVol := range desired {
		currentVol, exists := currentVolumes[desiredVol.Name]
		if !exists {
			return true
		}

		// Compare ConfigMap sources if they exist
		if desiredVol.ConfigMap != nil && currentVol.ConfigMap != nil {
			if desiredVol.ConfigMap.Name != currentVol.ConfigMap.Name {
				return true
			}

			// Compare items
			if len(desiredVol.ConfigMap.Items) != len(currentVol.ConfigMap.Items) {
				return true
			}

			// This part compares the actual content of items
			// Create map of current items for comparison
			currentItems := make(map[string]string)
			for _, item := range currentVol.ConfigMap.Items {
				currentItems[item.Key] = item.Path
			}

			for _, item := range desiredVol.ConfigMap.Items {
				path, exists := currentItems[item.Key]
				if !exists || path != item.Path {
					return true
				}
			}
		} else if (desiredVol.ConfigMap == nil) != (currentVol.ConfigMap == nil) {
			// One has a ConfigMap and the other doesn't
			return true
		}

		// Add additional comparisons for other volume source types if needed
	}

	return false
}

// Helper function to compare volume mounts
func compareVolumeMounts(current, desired []corev1.VolumeMount) bool {
	if len(current) != len(desired) {
		return true
	}

	// Create maps for easier comparison
	currentMounts := make(map[string]corev1.VolumeMount)
	for _, vm := range current {
		currentMounts[vm.Name] = vm
	}

	for _, desiredMount := range desired {
		currentMount, exists := currentMounts[desiredMount.Name]
		if !exists {
			return true
		}

		// Compare important fields
		if desiredMount.MountPath != currentMount.MountPath ||
			desiredMount.SubPath != currentMount.SubPath ||
			desiredMount.ReadOnly != currentMount.ReadOnly {
			return true
		}
	}

	return false
}

func (r *LogstashReconciler) findLogstashStateChanges(ctx context.Context, logstash *logstashv1alpha1.Logstash) (bool, *ctrl.Result, error) {
	// Update the Logstash status with the pod names
	// List the pods for this logstash's stateful set
	log := ctrllog.FromContext(ctx)
	log.Info("finding state changes")

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
	log.Info("update state")

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the latest version of Logstash before updating
		latestLogstash := &logstashv1alpha1.Logstash{}
		if err := r.Get(ctx, types.NamespacedName{Name: logstash.Name, Namespace: logstash.Namespace}, latestLogstash); err != nil {
			return err
		}

		// Update the status
		latestLogstash.Status = logstash.Status

		// Try to update
		return r.Status().Update(ctx, latestLogstash)
	})

	if err != nil {
		log.Error(err, "Failed to update Logstash status")
		return &ctrl.Result{}, err
	}
	return nil, nil
}

// statefulsetForLogstash returns a logstash Statefulset object
func (r *LogstashReconciler) statefulsetForLogstash(ctx context.Context, m *logstashv1alpha1.Logstash) (*appsv1.StatefulSet, error) {
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
				Resources: corev1.VolumeResourceRequirements{
					Requests: resources,
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{},
		},
	}

	logstashConfigMap := r.configMapForLogstash(m)
	if err := r.Create(ctx, logstashConfigMap); err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	pipelineList := &logstashv1alpha1.LogstashPipelineList{}
	if err := r.List(ctx, pipelineList, client.InNamespace(m.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list LogstashPipelines: %w", err)
	}

	// Create volume mounts for each pipeline
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "pipelines-yml",
			MountPath: "/usr/share/logstash/config/pipelines.yml",
			SubPath:   "pipelines.yml",
		},
		{
			Name:      "logstash-config",
			MountPath: "/usr/share/logstash/config/logstash.yml",
			SubPath:   "logstash.yml",
		},
	}

	// Create volumes for each pipeline
	volumes := []corev1.Volume{
		{
			Name: "pipelines-yml",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-pipelines-yml", m.Name),
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "pipelines.yml",
							Path: "pipelines.yml",
						},
					},
				},
			},
		},
		{
			Name: "logstash-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: logstashConfigMap.Name,
					},
				},
			},
		},
	}

	// Add volume mounts and volumes for each pipeline
	for _, pipeline := range pipelineList.Items {
		// Add volume mount for this pipeline
		volumeMount := corev1.VolumeMount{
			Name:      fmt.Sprintf("pipeline-%s", pipeline.Name),
			MountPath: fmt.Sprintf("/usr/share/logstash/pipeline-%s", pipeline.Name),
		}
		volumeMounts = append(volumeMounts, volumeMount)

		// Add volume for this pipeline
		volume := corev1.Volume{
			Name: fmt.Sprintf("pipeline-%s", pipeline.Name),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-pipeline-%s-config", m.Name, pipeline.Name),
					},
				},
			},
		}
		volumes = append(volumes, volume)
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
					Containers: []corev1.Container{
						{
							Image: "logstash:8.17.2",
							Name:  "logstash",
							Ports: []corev1.ContainerPort{{
								ContainerPort: 9600,
								Name:          "logstash",
							}},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
			VolumeClaimTemplates: pvcs,
			ServiceName:          fmt.Sprintf("%s-headless", m.Name),
		},
	}
	// Set Logstash instance as the owner and controller
	err := ctrl.SetControllerReference(m, sfs, r.Scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to set ControllerReference: %w", err)
	}
	return sfs, nil
}

func (r *LogstashReconciler) configMapForLogstash(m *logstashv1alpha1.Logstash) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", m.Name),
			Namespace: m.Namespace,
		},
		Data: map[string]string{
			"logstash.yml": fmt.Sprintf("config.reload.automatic: %t\nconfig.reload.interval: %ds", m.Spec.ConfigReload.Automatic, m.Spec.ConfigReload.Interval),
		},
	}
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
