package controllers

import (
	"context"
	"fmt"
	logstashv1alpha1 "github.com/hendrikkiedrowski/logstash-operator-go/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
	"sort"
	"strings"
)

type PipleneYmlStruct struct {
	PipelineID string `yaml:"pipeline.id"`
	PathConfig string `yaml:"path.config,omitempty"`
}

func (r *LogstashReconciler) reconcilePipelineConfigMap(ctx context.Context, logstash *logstashv1alpha1.Logstash) error {
	//log := ctrllog.FromContext(ctx)
	pipelineList := &logstashv1alpha1.LogstashPipelineList{}
	if err := r.List(ctx, pipelineList, client.InNamespace(logstash.Namespace)); err != nil {
		return fmt.Errorf("failed to list LogstashPipelines: %w", err)
	}

	// Generate pipelines.yml
	pipelinesYML := generatePipelinesYML(ctx, pipelineList.Items)
	pipelinesYMLConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-pipelines-yml", logstash.Name),
			Namespace: logstash.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(logstash, logstashv1alpha1.GroupVersion.WithKind("Logstash")),
			},
		},
		Data: map[string]string{
			"pipelines.yml": pipelinesYML,
		},
	}

	if err := r.createOrUpdateConfigMap(ctx, pipelinesYMLConfigMap); err != nil {
		return fmt.Errorf("failed to reconcile pipelines.yml ConfigMap: %w", err)
	}

	// Create a ConfigMap for each pipeline's configuration files
	for _, pipeline := range pipelineList.Items {
		// Create a map to store all config files for this pipeline
		pipelineConfigData := make(map[string]string)

		// Generate input configuration
		inputConfig, err := r.generateInputConfig(ctx, &pipeline)
		if err != nil {
			return fmt.Errorf("failed to generate input config for %s: %w", pipeline.Name, err)
		}
		pipelineConfigData["input.conf"] = inputConfig

		// Generate output configuration
		outputConfig, err := r.generateOutputConfig(ctx, &pipeline)
		if err != nil {
			return fmt.Errorf("failed to generate output config for %s: %w", pipeline.Name, err)
		}
		pipelineConfigData["output.conf"] = outputConfig

		// Generate filter configurations
		filterConfigs, err := r.generateFilterConfigs(ctx, &pipeline)
		if err != nil {
			return fmt.Errorf("failed to generate filter configs for %s: %w", pipeline.Name, err)
		}

		// Add filter configurations to the pipeline config data
		for filename, content := range filterConfigs {
			pipelineConfigData[filename] = content
		}

		// Create or update the ConfigMap for this pipeline
		pipelineConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-pipeline-%s-config", logstash.Name, pipeline.Name),
				Namespace: logstash.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(logstash, logstashv1alpha1.GroupVersion.WithKind("Logstash")),
				},
			},
			Data: pipelineConfigData,
		}

		if err = r.createOrUpdateConfigMap(ctx, pipelineConfigMap); err != nil {
			return fmt.Errorf("failed to reconcile pipeline ConfigMap for %s: %w", pipeline.Name, err)
		}
	}

	return nil
}

// Generate input configuration for a pipeline
func (r *LogstashReconciler) generateInputConfig(ctx context.Context, pipeline *logstashv1alpha1.LogstashPipeline) (string, error) {
	var config strings.Builder

	// Convert LabelSelector to Selector
	selector, err := metav1.LabelSelectorAsSelector(pipeline.Spec.Selector)
	if err != nil {
		return "", fmt.Errorf("failed to create selector: %w", err)
	}

	// Generate input configuration
	config.WriteString("input {\n")
	inputList := &logstashv1alpha1.LogstashInputList{}
	if err := r.List(ctx, inputList, client.InNamespace(pipeline.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return "", fmt.Errorf("failed to list inputs: %w", err)
	}
	for _, input := range inputList.Items {
		config.WriteString(input.Spec.Data)
		config.WriteString("\n")
	}
	config.WriteString("}\n")

	return config.String(), nil
}

// Generate output configuration for a pipeline
func (r *LogstashReconciler) generateOutputConfig(ctx context.Context, pipeline *logstashv1alpha1.LogstashPipeline) (string, error) {
	var config strings.Builder

	// Convert LabelSelector to Selector
	selector, err := metav1.LabelSelectorAsSelector(pipeline.Spec.Selector)
	if err != nil {
		return "", fmt.Errorf("failed to create selector: %w", err)
	}

	// Generate output configuration
	config.WriteString("output {\n")
	outputList := &logstashv1alpha1.LogstashOutputList{}
	if err := r.List(ctx, outputList, client.InNamespace(pipeline.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return "", fmt.Errorf("failed to list outputs: %w", err)
	}
	for _, output := range outputList.Items {
		config.WriteString(output.Spec.Data)
		config.WriteString("\n")
	}
	config.WriteString("}\n")

	return config.String(), nil
}

// Generate filter configurations for a pipeline
func (r *LogstashReconciler) generateFilterConfigs(ctx context.Context, pipeline *logstashv1alpha1.LogstashPipeline) (map[string]string, error) {
	log := ctrllog.FromContext(ctx)
	filterConfigs := make(map[string]string)

	// Convert LabelSelector to Selector
	selector, err := metav1.LabelSelectorAsSelector(pipeline.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("failed to create selector: %w", err)
	}

	// List filters
	filterList := &logstashv1alpha1.LogstashFilterList{}
	if err := r.List(ctx, filterList, client.InNamespace(pipeline.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, fmt.Errorf("failed to list filters: %w", err)
	}

	// Sort filters by order
	sort.Slice(filterList.Items, func(i, j int) bool {
		return filterList.Items[i].Spec.Order < filterList.Items[j].Spec.Order
	})

	// Process each filter
	for _, filter := range filterList.Items {
		filterFileName := fmt.Sprintf("%d-%s.conf", filter.Spec.Order, filter.Name)

		// Create filter content with filter block
		var filterContent strings.Builder
		filterContent.WriteString("filter {\n")
		filterContent.WriteString(filter.Spec.Data)
		filterContent.WriteString("\n}\n")

		filterConfigs[filterFileName] = filterContent.String()
		log.Info("Added filter configuration", "pipeline", pipeline.Name, "filter", filter.Name, "fileName", filterFileName)
	}

	return filterConfigs, nil
}

func (r *LogstashReconciler) createOrUpdateConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	log := ctrllog.FromContext(ctx)

	existingCM := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, existingCM)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("Creating ConfigMap", "name", cm.Name)
			return r.Create(ctx, cm)
		}
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}
	needsUpdate := false
	if strings.Contains(cm.Name, "pipelines-yml") {
		needsUpdate, err = r.comparePipelinesYML(existingCM.Data["pipelines.yml"], cm.Data["pipelines.yml"])
		if err != nil {
			return fmt.Errorf("error comparing pipelines.yml: %w", err)
		}
		if needsUpdate {
			log.Info("Updating ConfigMap", "name", cm.Name)
			existingCM.Data = cm.Data // Update in-place for efficiency
			if err := r.Update(ctx, existingCM); err != nil {
				return fmt.Errorf("failed to update ConfigMap: %w", err)
			}
			return nil
		}

	} else if !reflect.DeepEqual(existingCM.Data, cm.Data) {
		existingCM.Data = cm.Data
		log.Info("Updating ConfigMap", "name", cm.Name)
		if err := r.Update(ctx, existingCM); err != nil {
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}
		return nil
	}

	log.Info("ConfigMap is up to date", "name", cm.Name)
	return nil
}

func generatePipelinesYML(ctx context.Context, pipelines []logstashv1alpha1.LogstashPipeline) string {
	log := ctrllog.FromContext(ctx)
	var sb strings.Builder
	sb.WriteString("# This file is generated automatically by the Logstash Operator\n")
	sb.WriteString("# Do not edit this file directly\n\n")

	for _, pipeline := range pipelines {
		sb.WriteString(fmt.Sprintf("- pipeline.id: %s\n", pipeline.Name))
		sb.WriteString(fmt.Sprintf("  path.config: \"/usr/share/logstash/pipeline-%s\"\n", pipeline.Name))
		log.Info("Added pipeline to pipelines.yml", "pipelineName", pipeline.Name)
	}
	return sb.String()
}

func (r *LogstashReconciler) generatePipelineConfig(ctx context.Context, pipeline *logstashv1alpha1.LogstashPipeline) (string, error) {
	var config strings.Builder

	// Convert LabelSelector to Selector
	selector, err := metav1.LabelSelectorAsSelector(pipeline.Spec.Selector)
	if err != nil {
		return "", fmt.Errorf("failed to create selector: %w", err)
	}

	// Generate input configuration
	config.WriteString("input {\n")
	inputList := &logstashv1alpha1.LogstashInputList{}
	if err := r.List(ctx, inputList, client.InNamespace(pipeline.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return "", fmt.Errorf("failed to list inputs: %w", err)
	}
	for _, input := range inputList.Items {
		config.WriteString(input.Spec.Data)
		config.WriteString("\n")
	}
	config.WriteString("}\n\n")

	// Generate filter configuration
	config.WriteString("filter {\n")
	filterList := &logstashv1alpha1.LogstashFilterList{}
	if err := r.List(ctx, filterList, client.InNamespace(pipeline.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return "", fmt.Errorf("failed to list filters: %w", err)
	}

	sort.Slice(filterList.Items, func(i, j int) bool {
		return filterList.Items[i].Spec.Order < filterList.Items[j].Spec.Order
	})

	for _, filter := range filterList.Items {
		if filter.Spec.FromFile {
			continue
		} else {
			// Inline filter data as before
			config.WriteString(filter.Spec.Data)
			config.WriteString("\n")
		}
	}
	config.WriteString("}\n\n")

	// Generate output configuration
	config.WriteString("output {\n")
	outputList := &logstashv1alpha1.LogstashOutputList{}
	if err := r.List(ctx, outputList, client.InNamespace(pipeline.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return "", fmt.Errorf("failed to list outputs: %w", err)
	}
	for _, output := range outputList.Items {
		config.WriteString(output.Spec.Data)
		config.WriteString("\n")
	}
	config.WriteString("}\n")

	return config.String(), nil
}

func (r *LogstashReconciler) comparePipelinesYML(existingYAML, newYAML string) (bool, error) {
	if existingYAML == newYAML { // Fast path: strings are identical
		return false, nil
	}

	existingPipelines, err := parsePipelinesYAML(existingYAML)
	if err != nil {
		return false, fmt.Errorf("failed to parse existing pipelines.yml: %w", err)
	}
	newPipelines, err := parsePipelinesYAML(newYAML)
	if err != nil {
		return false, fmt.Errorf("failed to parse new pipelines.yml: %w", err)
	}

	if !arePipelineConfigsEqual(existingPipelines, newPipelines) {
		return true, nil // Needs update if pipeline configurations are different
	}

	return false, nil // No update needed if pipeline configurations are the same
}

func parsePipelinesYAML(content string) ([]PipleneYmlStruct, error) {
	if content == "" { // Handle empty pipelines.yml
		return nil, nil
	}

	var pipelines []PipleneYmlStruct
	err := yaml.Unmarshal([]byte(content), &pipelines) // Directly unmarshal into a slice
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pipelines.yml: %w", err)
	}
	return pipelines, nil
}

func arePipelineConfigsEqual(existing, new []PipleneYmlStruct) bool {
	if len(existing) != len(new) {
		return false
	}

	// Create maps for efficient lookup by PipelineID
	existingMap := make(map[string]PipleneYmlStruct, len(existing))
	for _, p := range existing {
		existingMap[p.PipelineID] = p
	}
	newMap := make(map[string]PipleneYmlStruct, len(new))
	for _, p := range new {
		newMap[p.PipelineID] = p
	}

	if len(existingMap) != len(newMap) { // Double check length after map creation
		return false // Should not happen unless duplicate IDs in input, but good to check
	}

	for pipelineID, existingPipeline := range existingMap {
		newPipeline, ok := newMap[pipelineID]
		if !ok {
			return false // Pipeline ID missing in new config
		}
		if !reflect.DeepEqual(existingPipeline, newPipeline) {
			return false // Pipeline configurations are different
		}
	}

	return true // All pipelines are present and equal
}
