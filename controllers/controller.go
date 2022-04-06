package controllers

import (
	"context"

	logstashv1alpha1 "github.com/hendrikkiedrowski/logstash-operator-go/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MasterReconciler struct {
	logstashReconciler LogstashReconciler
	client             client.Client
	scheme             *runtime.Scheme
}

func NewMasterReconciler(mgr ctrl.Manager) *MasterReconciler {
	client := mgr.GetClient()
	scheme := mgr.GetScheme()
	logStashController := &LogstashReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return &MasterReconciler{
		logstashReconciler: *logStashController,
		client:             client,
		scheme:             scheme,
	}
}

func (m *MasterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return m.logstashReconciler.Reconcile(ctx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MasterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&logstashv1alpha1.Logstash{}).
		Complete(r)
}
