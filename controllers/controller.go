package controllers

import (
	"context"
	logstashv1alpha1 "github.com/hendrikkiedrowski/logstash-operator-go/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
func (m *MasterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	log := ctrllog.FromContext(context.Background())
	log.Info("Setting up controller SetupWithManager")
	return ctrl.NewControllerManagedBy(mgr).
		For(&logstashv1alpha1.Logstash{}).
		Watches(
			&logstashv1alpha1.LogstashPipeline{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
				return m.findLogstashForPipeline(o)
			}),
			//builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(&logstashv1alpha1.LogstashInput{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
				return m.findLogstashForInput(o)
			}),
			//builder.WithPredicates(predicate.GenerationChangedPredicate{})
		).
		Watches(&logstashv1alpha1.LogstashOutput{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
				return m.findLogstashForOutput(o)
			}),
			//builder.WithPredicates(predicate.GenerationChangedPredicate{})
		).
		Watches(&logstashv1alpha1.LogstashFilter{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
				return m.findLogstashForFilter(o)
			}),
			//builder.WithPredicates(predicate.GenerationChangedPredicate{})
		).
		Complete(m)
}

// findLogstashForPipeline is a mapping function to trigger Logstash reconciliation when a LogstashPipeline changes.
func (m *MasterReconciler) findLogstashForPipeline(o client.Object) []ctrl.Request { // Receiver is *MasterReconciler
	pipeline := o.(*logstashv1alpha1.LogstashPipeline)
	log := ctrllog.FromContext(context.Background())
	log.Info("Reconciling Logstash due to LogstashPipeline change", "pipeline", pipeline.Name, "namespace", pipeline.Namespace)

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "logstash-sample",
				Namespace: pipeline.Namespace,
			},
		},
	}
}

// findLogstashForInput is a mapping function to trigger Logstash reconciliation when a LogstashInput changes.
func (m *MasterReconciler) findLogstashForInput(o client.Object) []ctrl.Request { // Receiver is *MasterReconciler
	input := o.(*logstashv1alpha1.LogstashInput)
	log := ctrllog.FromContext(context.Background())
	log.Info("Reconciling Logstash due to LogstashInput change", "input", input.Name, "namespace", input.Namespace)

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "logstash-sample",
				Namespace: input.Namespace,
			},
		},
	}
}

// findLogstashForOutput is a mapping function to trigger Logstash reconciliation when a LogstashOutput changes.
func (m *MasterReconciler) findLogstashForOutput(o client.Object) []ctrl.Request { // Receiver is *MasterReconciler
	output := o.(*logstashv1alpha1.LogstashOutput)
	log := ctrllog.FromContext(context.Background())
	log.Info("Reconciling Logstash due to LogstashOutput change", "output", output.Name, "namespace", output.Namespace)

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "logstash-sample",
				Namespace: output.Namespace,
			},
		},
	}
}

// findLogstashForFilter is a mapping function to trigger Logstash reconciliation when a LogstashFilter changes.
func (m *MasterReconciler) findLogstashForFilter(o client.Object) []ctrl.Request { // Receiver is *MasterReconciler
	filter := o.(*logstashv1alpha1.LogstashFilter)
	log := ctrllog.FromContext(context.Background())
	log.Info("Reconciling Logstash due to LogstashFilter change", "filter", filter.Name, "namespace", filter.Namespace)

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "logstash-sample",
				Namespace: filter.Namespace,
			},
		},
	}
}
