/*
Copyright 2022 The Numaproj Authors.

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

package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/imdario/mergo"
	"go.uber.org/zap"
	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	dfv1 "github.com/numaproj/numaflow/pkg/apis/numaflow/v1alpha1"
	daemonclient "github.com/numaproj/numaflow/pkg/daemon/client"
	"github.com/numaproj/numaflow/pkg/metrics"
	"github.com/numaproj/numaflow/pkg/reconciler"
	"github.com/numaproj/numaflow/pkg/shared/logging"
	sharedutil "github.com/numaproj/numaflow/pkg/shared/util"
)

const (
	finalizerName = dfv1.ControllerPipeline

	pauseTimestampPath = `/metadata/annotations/numaflow.numaproj.io~1pause-timestamp`
)

// pipelineReconciler reconciles a pipeline object.
type pipelineReconciler struct {
	client client.Client
	scheme *runtime.Scheme

	config   *reconciler.GlobalConfig
	image    string
	logger   *zap.SugaredLogger
	recorder record.EventRecorder
}

func NewReconciler(client client.Client, scheme *runtime.Scheme, config *reconciler.GlobalConfig, image string, logger *zap.SugaredLogger, recorder record.EventRecorder) reconcile.Reconciler {
	return &pipelineReconciler{client: client, scheme: scheme, config: config, image: image, logger: logger, recorder: recorder}
}

func (r *pipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pl := &dfv1.Pipeline{}
	if err := r.client.Get(ctx, req.NamespacedName, pl); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		r.logger.Errorw("Unable to get pipeline", zap.Any("request", req), zap.Error(err))
		return ctrl.Result{}, err
	}
	log := r.logger.With("namespace", pl.Namespace).With("pipeline", pl.Name)
	plCopy := pl.DeepCopy()
	ctx = logging.WithLogger(ctx, log)
	result, reconcileErr := r.reconcile(ctx, plCopy)
	if reconcileErr != nil {
		log.Errorw("Reconcile error", zap.Error(reconcileErr))
	}
	plCopy.Status.LastUpdated = metav1.Now()
	if !equality.Semantic.DeepEqual(pl.Finalizers, plCopy.Finalizers) {
		patchYaml := "metadata:\n  finalizers: [" + strings.Join(plCopy.Finalizers, ",") + "]"
		patchJson, _ := yaml.YAMLToJSON([]byte(patchYaml))
		if err := r.client.Patch(ctx, pl, client.RawPatch(types.MergePatchType, []byte(patchJson))); err != nil {
			return result, err
		}
	}
	if err := r.client.Status().Update(ctx, plCopy); err != nil {
		return result, err
	}
	return result, reconcileErr
}

// reconcile does the real logic
func (r *pipelineReconciler) reconcile(ctx context.Context, pl *dfv1.Pipeline) (ctrl.Result, error) {
	log := logging.FromContext(ctx)
	if !pl.DeletionTimestamp.IsZero() {
		log.Info("Deleting pipeline")
		if controllerutil.ContainsFinalizer(pl, finalizerName) {
			if time.Now().Before(pl.DeletionTimestamp.Add(time.Duration(pl.Spec.Lifecycle.GetDeleteGracePeriodSeconds()) * time.Second)) {
				safeToDelete, err := r.safeToDelete(ctx, pl)
				if err != nil {
					logMsg := fmt.Sprintf("Failed to check if it's safe to delete pipeline %s: %v", pl.Name, err.Error())
					log.Error(logMsg)
					return ctrl.Result{}, err
				}

				if !safeToDelete {
					log.Info("Pipeline deletion is waiting to finish the unconsumed messages")
					// Requeue request to process after 10s
					return ctrl.Result{RequeueAfter: dfv1.DefaultRequeueAfter}, nil
				}
			}
			// Finalizer logic should be added here.
			if err := r.cleanUpBuffers(ctx, pl, log); err != nil {
				logMsg := fmt.Sprintf("Failed to create buffer clean up job: %v", err.Error())
				log.Error(logMsg)
				r.recorder.Event(pl, corev1.EventTypeWarning, "ReconcilePipelineFailed", logMsg)
				return ctrl.Result{}, err

			}
			controllerutil.RemoveFinalizer(pl, finalizerName)
			// Clean up metrics
			_ = reconciler.PipelineHealth.DeleteLabelValues(pl.Namespace, pl.Name)
			// Delete corresponding vertex metrics
			_ = reconciler.VertexDesiredReplicas.DeletePartialMatch(map[string]string{metrics.LabelNamespace: pl.Namespace, metrics.LabelPipeline: pl.Name})
			_ = reconciler.VertexCurrentReplicas.DeletePartialMatch(map[string]string{metrics.LabelNamespace: pl.Namespace, metrics.LabelPipeline: pl.Name})
		}
		return ctrl.Result{}, nil
	}

	defer func() {
		if pl.Status.IsHealthy() {
			reconciler.PipelineHealth.WithLabelValues(pl.Namespace, pl.Name).Set(1)
		} else {
			reconciler.PipelineHealth.WithLabelValues(pl.Namespace, pl.Name).Set(0)
		}
	}()

	pl.Status.InitConditions()
	pl.Status.SetObservedGeneration(pl.Generation)

	if !controllerutil.ContainsFinalizer(pl, finalizerName) {
		controllerutil.AddFinalizer(pl, finalizerName)
	}
	if err := ValidatePipeline(pl); err != nil {
		r.recorder.Eventf(pl, corev1.EventTypeWarning, "ValidatePipelineFailed", "Invalid pipeline: %s", err.Error())
		pl.Status.MarkNotConfigured("InvalidSpec", err.Error())
		return ctrl.Result{}, err
	}
	pl.Status.SetVertexCounts(pl.Spec.Vertices)
	pl.Status.MarkConfigured()

	// Orchestrate pipeline sub resources.
	// This should be happening in all cases to ensure a clean initialization regardless of the lifecycle phase.
	// Eg: even for a pipeline started with desiredPhase = Pause, we should still create the resources for the pipeline.
	if err := r.reconcileFixedResources(ctx, pl); err != nil {
		r.recorder.Eventf(pl, corev1.EventTypeWarning, "ReconcileFixedResourcesFailed", "Failed to reconcile pipeline sub resources: %s", err.Error())
		pl.Status.MarkDeployFailed("ReconcileFixedResourcesFailed", err.Error())
		return ctrl.Result{}, err
	}
	pl.Status.MarkDeployed()

	// If the pipeline has a lifecycle change, then do not update the phase as
	// this should happen only after the required configs for the lifecycle changes
	// have been applied.
	if !isLifecycleChange(pl) {
		pl.Status.SetPhase(pl.Spec.Lifecycle.GetDesiredPhase(), "")
	}
	if err := r.checkChildrenResourceStatus(ctx, pl); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check pipeline children resource status, %w", err)
	}

	// check if any changes related to pause/resume lifecycle for the pipeline
	if isLifecycleChange(pl) {
		oldPhase := pl.Status.Phase
		requeue, err := r.updateDesiredState(ctx, pl)
		if err != nil {
			logMsg := fmt.Sprintf("Updated desired pipeline phase failed: %v", zap.Error(err))
			log.Error(logMsg)
			r.recorder.Eventf(pl, corev1.EventTypeWarning, "ReconcilePipelineFailed", logMsg)
			return ctrl.Result{}, err
		}
		if pl.Status.Phase != oldPhase {
			log.Infow("Updated pipeline phase", zap.String("originalPhase", string(oldPhase)), zap.String("currentPhase", string(pl.Status.Phase)))
			r.recorder.Eventf(pl, corev1.EventTypeNormal, "UpdatePipelinePhase", "Updated pipeline phase from %s to %s", string(oldPhase), string(pl.Status.Phase))
		}
		if requeue {
			return ctrl.Result{RequeueAfter: dfv1.DefaultRequeueAfter}, nil
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// isLifecycleChange determines whether there has been a change requested in the lifecycle
// of a Pipeline object, specifically relating to the paused and pausing states.
func isLifecycleChange(pl *dfv1.Pipeline) bool {
	// Extract the current phase from the status of the pipeline.
	// Check if the desired phase of the pipeline is 'Paused', or if the current phase of the
	// pipeline is either 'Paused' or 'Pausing'. This indicates a transition into or out of
	// a paused state which is a lifecycle phase change
	if oldPhase := pl.Status.Phase; pl.Spec.Lifecycle.GetDesiredPhase() == dfv1.PipelinePhasePaused ||
		oldPhase == dfv1.PipelinePhasePaused || oldPhase == dfv1.PipelinePhasePausing {
		return true
	}

	// If none of the conditions are met, return false
	return false
}

// reconcileFixedResources do the jobs of creating fixed resources such as daemon service, vertex objects, and ISB management jobs, etc
func (r *pipelineReconciler) reconcileFixedResources(ctx context.Context, pl *dfv1.Pipeline) error {
	log := logging.FromContext(ctx)
	isbSvc := &dfv1.InterStepBufferService{}
	isbSvcName := dfv1.DefaultISBSvcName
	if len(pl.Spec.InterStepBufferServiceName) > 0 {
		isbSvcName = pl.Spec.InterStepBufferServiceName
	}
	err := r.client.Get(ctx, types.NamespacedName{Namespace: pl.Namespace, Name: isbSvcName}, isbSvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Errorw("ISB Service not found", zap.String("isbsvc", isbSvcName), zap.Error(err))
			return fmt.Errorf("isbsvc %s not found", isbSvcName)
		}
		log.Errorw("Failed to get ISB Service", zap.String("isbsvc", isbSvcName), zap.Error(err))
		return err
	}
	if !isbSvc.Status.IsHealthy() {
		log.Errorw("ISB Service is not in healthy status", zap.String("isbsvc", isbSvcName), zap.Error(err))
		return fmt.Errorf("isbsvc not healthy")
	}

	// Create or update the Side Inputs Manager deployments
	if err := r.createOrUpdateSIMDeployments(ctx, pl, isbSvc.Status.Config); err != nil {
		log.Errorw("Failed to create or update Side Inputs Manager deployments", zap.Error(err))
		r.recorder.Eventf(pl, corev1.EventTypeWarning, "CreateOrUpdateSIMDeploymentsFailed", "Failed to create or update Side Inputs Manager deployments: %w", err.Error())
		return fmt.Errorf("failed to create or update SIM deployments: %w", err)
	}

	existingObjs, err := r.findExistingVertices(ctx, pl)
	if err != nil {
		return fmt.Errorf("failed to find existing vertices: %w", err)
	}
	oldBuffers := make(map[string]string)
	newBuffers := make(map[string]string)
	oldBuckets := make(map[string]string)
	newBuckets := make(map[string]string)
	for _, v := range existingObjs {
		for _, b := range v.OwnedBuffers() {
			oldBuffers[b] = b
		}
		for _, b := range v.GetFromBuckets() {
			oldBuckets[b] = b
		}
		for _, b := range v.GetToBuckets() {
			oldBuckets[b] = b
		}
	}
	for _, b := range pl.GetAllBuffers() {
		if _, existing := oldBuffers[b]; existing {
			delete(oldBuffers, b)
		} else {
			newBuffers[b] = b
		}
	}
	for _, b := range pl.GetAllBuckets() {
		if _, existing := oldBuckets[b]; existing {
			delete(oldBuckets, b)
		} else {
			newBuckets[b] = b
		}
	}
	newObjs := buildVertices(pl)
	for vertexName, newObj := range newObjs {
		if oldObj, existing := existingObjs[vertexName]; !existing {
			if err := r.client.Create(ctx, &newObj); err != nil {
				if apierrors.IsAlreadyExists(err) { // probably somebody else already created it
					continue
				} else {
					r.recorder.Eventf(pl, corev1.EventTypeWarning, "CreateVertexFailed", "Failed to create vertex: %w", err.Error())
					return fmt.Errorf("failed to create vertex, err: %w", err)
				}
			}
			log.Infow("Created vertex successfully", zap.String("vertex", vertexName))
			r.recorder.Eventf(pl, corev1.EventTypeNormal, "CreateVertexSuccess", "Created vertex %s successfully", vertexName)
		} else {
			if oldObj.GetAnnotations()[dfv1.KeyHash] != newObj.GetAnnotations()[dfv1.KeyHash] { // need to update
				originalReplicas := oldObj.Spec.Replicas
				oldObj.Spec = newObj.Spec
				oldObj.Spec.Replicas = originalReplicas
				oldObj.Annotations[dfv1.KeyHash] = newObj.GetAnnotations()[dfv1.KeyHash]
				if err := r.client.Update(ctx, &oldObj); err != nil {
					r.recorder.Eventf(pl, corev1.EventTypeWarning, "UpdateVertexFailed", "Failed to update vertex: %w", err.Error())
					return fmt.Errorf("failed to update vertex, err: %w", err)
				}
				log.Infow("Updated vertex successfully", zap.String("vertex", vertexName))
				r.recorder.Eventf(pl, corev1.EventTypeNormal, "UpdateVertexSuccess", "Updated vertex %s successfully", vertexName)
			}
			delete(existingObjs, vertexName)
		}
	}
	for _, v := range existingObjs {
		if err := r.client.Delete(ctx, &v); err != nil {
			r.recorder.Eventf(pl, corev1.EventTypeWarning, "DeleteStaleVertexFailed", "Failed to delete vertex: %w", err.Error())
			return fmt.Errorf("failed to delete vertex, err: %w", err)
		}
		log.Infow("Deleted stale vertex successfully", zap.String("vertex", v.Name))
		r.recorder.Eventf(pl, corev1.EventTypeNormal, "DeleteStaleVertexSuccess", "Deleted stale vertex %s successfully", v.Name)
		// Clean up vertex replica metrics
		reconciler.VertexDesiredReplicas.DeleteLabelValues(pl.Namespace, pl.Name, v.Spec.Name)
		reconciler.VertexCurrentReplicas.DeleteLabelValues(pl.Namespace, pl.Name, v.Spec.Name)
	}

	// create batch job
	if len(newBuffers) > 0 || len(newBuckets) > 0 {
		bfs := []string{}
		for k := range newBuffers {
			bfs = append(bfs, k)
		}
		bks := []string{}
		for k := range newBuckets {
			bks = append(bks, k)
		}
		args := []string{fmt.Sprintf("--buffers=%s", strings.Join(bfs, ",")), fmt.Sprintf("--buckets=%s", strings.Join(bks, ","))}
		args = append(args, fmt.Sprintf("--side-inputs-store=%s", pl.GetSideInputsStoreName()))
		args = append(args, fmt.Sprintf("--serving-source-streams=%s", strings.Join(pl.GetServingSourceStreamNames(), ",")))
		batchJob := buildISBBatchJob(pl, r.image, isbSvc.Status.Config, "isbsvc-create", args, "cre")
		if err := r.client.Create(ctx, batchJob); err != nil && !apierrors.IsAlreadyExists(err) {
			r.recorder.Eventf(pl, corev1.EventTypeWarning, "CreateJobForISBCeationFailed", "Failed to create a Job: %w", err.Error())
			return fmt.Errorf("failed to create ISB creating job, err: %w", err)
		}
		log.Infow("Created a job successfully for ISB creating", zap.Any("buffers", bfs), zap.Any("buckets", bks), zap.Any("servingStreams", pl.GetServingSourceStreamNames()))
		r.recorder.Eventf(pl, corev1.EventTypeNormal, "CreateJobForISBCeationSuccessful", "Create ISB creation job successfully")
	}

	if len(oldBuffers) > 0 || len(oldBuckets) > 0 {
		bfs := []string{}
		for k := range oldBuffers {
			bfs = append(bfs, k)
		}
		bks := []string{}
		for k := range oldBuckets {
			bks = append(bks, k)
		}
		args := []string{fmt.Sprintf("--buffers=%s", strings.Join(bfs, ",")), fmt.Sprintf("--buckets=%s", strings.Join(bks, ","))}
		batchJob := buildISBBatchJob(pl, r.image, isbSvc.Status.Config, "isbsvc-delete", args, "del")
		if err := r.client.Create(ctx, batchJob); err != nil && !apierrors.IsAlreadyExists(err) {
			r.recorder.Eventf(pl, corev1.EventTypeWarning, "CreateJobForISBDeletionFailed", "Failed to create a Job: %w", err.Error())
			return fmt.Errorf("failed to create ISB deleting job, err: %w", err)
		}
		log.Infow("Created ISB Svc deleting job successfully", zap.Any("buffers", bfs), zap.Any("buckets", bks))
		r.recorder.Eventf(pl, corev1.EventTypeNormal, "CreateJobForISBDeletionSuccessful", "Create ISB deletion job successfully")
	}

	// Daemon service
	if err := r.createOrUpdateDaemonService(ctx, pl); err != nil {
		return err
	}
	// Daemon deployment
	if err := r.createOrUpdateDaemonDeployment(ctx, pl, isbSvc.Status.Config); err != nil {
		return err
	}

	return nil
}

func (r *pipelineReconciler) createOrUpdateDaemonService(ctx context.Context, pl *dfv1.Pipeline) error {
	log := logging.FromContext(ctx)
	svc := pl.GetDaemonServiceObj()
	svcHash := sharedutil.MustHash(svc.Spec)
	svc.Annotations = map[string]string{dfv1.KeyHash: svcHash}
	existingSvc := &corev1.Service{}
	needToCreatDaemonSvc := false
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: pl.Namespace, Name: svc.Name}, existingSvc); err != nil {
		if apierrors.IsNotFound(err) {
			needToCreatDaemonSvc = true
		} else {
			log.Errorw("Failed to find existing daemon service", zap.String("service", svc.Name), zap.Error(err))
			pl.Status.MarkDeployFailed("FindDaemonSvcFailed", err.Error())
			return fmt.Errorf("failed to find existing daemon service, %w", err)
		}
	} else if existingSvc.GetAnnotations()[dfv1.KeyHash] != svcHash {
		if err := r.client.Delete(ctx, existingSvc); err != nil && !apierrors.IsNotFound(err) {
			log.Errorw("Failed to delete existing daemon service", zap.String("service", existingSvc.Name), zap.Error(err))
			pl.Status.MarkDeployFailed("DelDaemonSvcFailed", err.Error())
			r.recorder.Eventf(pl, corev1.EventTypeWarning, "DelDaemonSvcFailed", "Failed to delete existing daemon service: %w", err.Error())
			return fmt.Errorf("failed to delete existing daemon service, %w", err)
		}
		needToCreatDaemonSvc = true
	}
	if needToCreatDaemonSvc {
		if err := r.client.Create(ctx, svc); err != nil {
			log.Errorw("Failed to create daemon service", zap.String("service", svc.Name), zap.Error(err))
			pl.Status.MarkDeployFailed("CreateDaemonSvcFailed", err.Error())
			r.recorder.Eventf(pl, corev1.EventTypeWarning, "CreateDaemonSvcFailed", "Failed to create daemon service: %w", err.Error())
			return fmt.Errorf("failed to create daemon service, %w", err)
		}
		log.Infow("Succeeded to create a daemon service", zap.String("service", svc.Name))
		r.recorder.Eventf(pl, corev1.EventTypeNormal, "CreateDaemonSvcSuccess", "Succeeded to create daemon service %s", svc.Name)
	}
	return nil
}

func (r *pipelineReconciler) createOrUpdateDaemonDeployment(ctx context.Context, pl *dfv1.Pipeline, isbSvcConfig dfv1.BufferServiceConfig) error {
	log := logging.FromContext(ctx)
	isbSvcType, envs := sharedutil.GetIsbSvcEnvVars(isbSvcConfig)
	envs = append(envs, corev1.EnvVar{Name: dfv1.EnvPipelineName, Value: pl.Name})

	req := dfv1.GetDaemonDeploymentReq{
		ISBSvcType:       isbSvcType,
		Image:            r.image,
		PullPolicy:       corev1.PullPolicy(sharedutil.LookupEnvStringOr(dfv1.EnvImagePullPolicy, "")),
		Env:              envs,
		DefaultResources: r.config.GetDefaults().GetDefaultContainerResources(),
	}
	deploy, err := pl.GetDaemonDeploymentObj(req)
	if err != nil {
		pl.Status.MarkDeployFailed("BuildDaemonDeployFailed", err.Error())
		return fmt.Errorf("failed to build daemon deployment spec, %w", err)
	}
	deployHash := sharedutil.MustHash(deploy.Spec)
	deploy.Annotations = map[string]string{dfv1.KeyHash: deployHash}
	existingDeploy := &appv1.Deployment{}
	needToCreate := false
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: pl.Namespace, Name: deploy.Name}, existingDeploy); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Errorw("Failed to find existing daemon deployment", zap.String("deployment", deploy.Name), zap.Error(err))
			pl.Status.MarkDeployFailed("FindDaemonDeployFailed", err.Error())
			return fmt.Errorf("failed to find existing daemon deployment, %w", err)
		} else {
			needToCreate = true
		}
	} else {
		if existingDeploy.GetAnnotations()[dfv1.KeyHash] != deployHash {
			// Delete and recreate, to avoid updating immutable fields problem.
			if err := r.client.Delete(ctx, existingDeploy); err != nil {
				log.Errorw("Failed to delete the outdated daemon deployment", zap.String("deployment", existingDeploy.Name), zap.Error(err))
				pl.Status.MarkDeployFailed("DeleteOldDaemonDeployFailed", err.Error())
				r.recorder.Eventf(pl, corev1.EventTypeWarning, "DeleteOldDaemonDeployFailed", "Failed to delete the outdated daemon deployment: %w", err.Error())
				return fmt.Errorf("failed to delete an outdated daemon deployment, %w", err)
			}
			needToCreate = true
		}
	}
	if needToCreate {
		if err := r.client.Create(ctx, deploy); err != nil && !apierrors.IsAlreadyExists(err) {
			log.Errorw("Failed to create a daemon deployment", zap.String("deployment", deploy.Name), zap.Error(err))
			pl.Status.MarkDeployFailed("CreateDaemonDeployFailed", err.Error())
			r.recorder.Eventf(pl, corev1.EventTypeWarning, "CreateDaemonDeployFailed", "Failed to create a daemon deployment: %w", err.Error())
			return fmt.Errorf("failed to create a daemon deployment, %w", err)
		}
		log.Infow("Succeeded to create/recreate a daemon deployment", zap.String("deployment", deploy.Name))
		r.recorder.Eventf(pl, corev1.EventTypeNormal, "CreateDaemonDeploySuccess", "Succeeded to create/recreate daemon deployment %s", deploy.Name)
	}
	return nil
}

func (r *pipelineReconciler) findExistingVertices(ctx context.Context, pl *dfv1.Pipeline) (map[string]dfv1.Vertex, error) {
	vertices := &dfv1.VertexList{}
	selector, _ := labels.Parse(dfv1.KeyPipelineName + "=" + pl.Name)
	if err := r.client.List(ctx, vertices, &client.ListOptions{Namespace: pl.Namespace, LabelSelector: selector}); err != nil {
		return nil, fmt.Errorf("failed to list existing vertices: %w", err)
	}
	result := make(map[string]dfv1.Vertex)
	for _, v := range vertices.Items {
		result[v.Name] = v
	}
	return result, nil
}

// Create or update Side Inputs Manager deployments
func (r *pipelineReconciler) createOrUpdateSIMDeployments(ctx context.Context, pl *dfv1.Pipeline, isbSvcConfig dfv1.BufferServiceConfig) error {
	log := logging.FromContext(ctx)
	isbSvcType, envs := sharedutil.GetIsbSvcEnvVars(isbSvcConfig)
	envs = append(envs, corev1.EnvVar{Name: dfv1.EnvPipelineName, Value: pl.Name})

	req := dfv1.GetSideInputDeploymentReq{
		ISBSvcType:       isbSvcType,
		Image:            r.image,
		PullPolicy:       corev1.PullPolicy(sharedutil.LookupEnvStringOr(dfv1.EnvImagePullPolicy, "")),
		Env:              envs,
		DefaultResources: r.config.GetDefaults().GetDefaultContainerResources(),
	}

	newObjs, err := pl.GetSideInputsManagerDeployments(req)
	if err != nil {
		pl.Status.MarkDeployFailed("BuildSIMObjsFailed", err.Error())
		return fmt.Errorf("failed to build Side Inputs Manager Deployments, %w", err)
	}
	existingObjs, err := r.findExistingSIMDeploys(ctx, pl)
	if err != nil {
		pl.Status.MarkDeployFailed("FindExistingSIMFailed", err.Error())
		return fmt.Errorf("failed to find existing Side Inputs Manager Deployments, %w", err)
	}
	for _, newObj := range newObjs {
		deployHash := sharedutil.MustHash(newObj.Spec)
		if newObj.Annotations == nil {
			newObj.Annotations = make(map[string]string)
		}
		newObj.Annotations[dfv1.KeyHash] = deployHash
		needToCreate := false
		if oldObj, existing := existingObjs[newObj.Name]; existing {
			if oldObj.GetAnnotations()[dfv1.KeyHash] != newObj.GetAnnotations()[dfv1.KeyHash] {
				// Delete and recreate, to avoid updating immutable fields problem.
				if err := r.client.Delete(ctx, &oldObj); err != nil {
					pl.Status.MarkDeployFailed("DeleteOldSIMDeploymentFailed", err.Error())
					return fmt.Errorf("failed to delete old Side Inputs Manager Deployment %q, %w", oldObj.Name, err)
				}
				needToCreate = true
			}
			delete(existingObjs, oldObj.Name)
		} else {
			needToCreate = true
		}
		if needToCreate {
			if err := r.client.Create(ctx, newObj); err != nil {
				if apierrors.IsAlreadyExists(err) { // probably somebody else already created it
					continue
				} else {
					pl.Status.MarkDeployFailed("CreateSIMDeploymentFailed", err.Error())
					return fmt.Errorf("failed to create/recreate Side Inputs Manager Deployment %q, %w", newObj.Name, err)
				}
			}
			log.Infow("Succeeded to create/recreate Side Inputs Manager Deployment", zap.String("deployment", newObj.Name))
			r.recorder.Event(pl, corev1.EventTypeNormal, "CreateSIMDeployment", "Succeeded to create/recreate Side Inputs Manager Deployment")
		}
	}
	for _, v := range existingObjs {
		if err := r.client.Delete(ctx, &v); err != nil {
			pl.Status.MarkDeployFailed("DeleteStaleSIMDeploymentFailed", err.Error())
			return fmt.Errorf("failed to delete stale Side Inputs Manager Deployment %q, %w", v.Name, err)
		}
		log.Infow("Deleted stale Side Inputs Manager Deployment successfully", zap.String("deployment", v.Name))
	}
	return nil
}

// Find existing Side Inputs Manager Deployment objects.
func (r *pipelineReconciler) findExistingSIMDeploys(ctx context.Context, pl *dfv1.Pipeline) (map[string]appv1.Deployment, error) {
	deployments := &appv1.DeploymentList{}
	selector, _ := labels.Parse(dfv1.KeyPipelineName + "=" + pl.Name + "," + dfv1.KeyComponent + "=" + dfv1.ComponentSideInputManager)
	if err := r.client.List(ctx, deployments, &client.ListOptions{Namespace: pl.Namespace, LabelSelector: selector}); err != nil {
		return nil, fmt.Errorf("failed to list existing Side Inputs Manager deployments: %w", err)
	}
	result := make(map[string]appv1.Deployment)
	for _, d := range deployments.Items {
		result[d.Name] = d
	}
	return result, nil
}

func (r *pipelineReconciler) cleanUpBuffers(ctx context.Context, pl *dfv1.Pipeline, log *zap.SugaredLogger) error {
	allBuffers := pl.GetAllBuffers()
	allBuckets := pl.GetAllBuckets()
	if len(allBuffers) > 0 || len(allBuckets) > 0 {
		isbSvc := &dfv1.InterStepBufferService{}
		isbSvcName := dfv1.DefaultISBSvcName
		if len(pl.Spec.InterStepBufferServiceName) > 0 {
			isbSvcName = pl.Spec.InterStepBufferServiceName
		}
		err := r.client.Get(ctx, types.NamespacedName{Namespace: pl.Namespace, Name: isbSvcName}, isbSvc)
		if err != nil {
			if apierrors.IsNotFound(err) { // somehow it doesn't need to clean up
				return nil
			}
			log.Errorw("Failed to get ISB Service", zap.String("isbsvc", isbSvcName), zap.Error(err))
			return err
		}

		args := []string{}
		args = append(args, fmt.Sprintf("--buffers=%s", strings.Join(allBuffers, ",")))
		args = append(args, fmt.Sprintf("--buckets=%s", strings.Join(allBuckets, ",")))
		args = append(args, fmt.Sprintf("--side-inputs-store=%s", pl.GetSideInputsStoreName()))
		args = append(args, fmt.Sprintf("--serving-source-streams=%s", strings.Join(pl.GetServingSourceStreamNames(), ",")))

		batchJob := buildISBBatchJob(pl, r.image, isbSvc.Status.Config, "isbsvc-delete", args, "cln")
		batchJob.OwnerReferences = []metav1.OwnerReference{}
		if err := r.client.Create(ctx, batchJob); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create buffer clean up job, err: %w", err)
		}
		log.Infow("Created buffer clean up job successfully", zap.Any("buffers", allBuffers))
	}
	return nil
}

func buildVertices(pl *dfv1.Pipeline) map[string]dfv1.Vertex {
	result := make(map[string]dfv1.Vertex)
	for _, v := range pl.Spec.Vertices {
		vertexFullName := pl.Name + "-" + v.Name
		matchLabels := map[string]string{
			dfv1.KeyManagedBy:    dfv1.ControllerPipeline,
			dfv1.KeyComponent:    dfv1.ComponentVertex,
			dfv1.KeyPartOf:       dfv1.Project,
			dfv1.KeyPipelineName: pl.Name,
			dfv1.KeyVertexName:   v.Name,
		}
		fromEdges := copyEdges(pl, pl.GetFromEdges(v.Name))
		toEdges := copyEdges(pl, pl.GetToEdges(v.Name))
		vCopy := v.DeepCopy()
		copyVertexTemplate(pl, vCopy)
		copyVertexLimits(pl, vCopy)
		replicas := int32(1)
		// If the desired phase is paused or we are in the middle of pausing we should not start any vertex replicas
		if isLifecycleChange(pl) {
			replicas = int32(0)
		} else if v.IsReduceUDF() {
			partitions := pl.NumOfPartitions(v.Name)
			replicas = int32(partitions)
		} else {
			x := vCopy.Scale
			if x.Min != nil && *x.Min > 1 && replicas < *x.Min {
				replicas = *x.Min
			}
			if x.Max != nil && *x.Max > 1 && replicas > *x.Max {
				replicas = *x.Max
			}
		}

		spec := dfv1.VertexSpec{
			AbstractVertex:             *vCopy,
			PipelineName:               pl.Name,
			InterStepBufferServiceName: pl.Spec.InterStepBufferServiceName,
			FromEdges:                  fromEdges,
			ToEdges:                    toEdges,
			Watermark:                  pl.Spec.Watermark,
			Replicas:                   &replicas,
		}
		hash := sharedutil.MustHash(spec.DeepCopyWithoutReplicas())
		obj := dfv1.Vertex{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: pl.Namespace,
				Name:      vertexFullName,
				Labels:    matchLabels,
				Annotations: map[string]string{
					dfv1.KeyHash: hash,
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(pl.GetObjectMeta(), dfv1.PipelineGroupVersionKind),
				},
			},
			Spec: spec,
		}
		result[obj.Name] = obj
	}
	return result
}

func copyVertexLimits(pl *dfv1.Pipeline, v *dfv1.AbstractVertex) {
	mergedLimits := mergeLimits(pl.GetPipelineLimits(), v.Limits)
	v.Limits = &mergedLimits
}

func mergeLimits(plLimits dfv1.PipelineLimits, vLimits *dfv1.VertexLimits) dfv1.VertexLimits {
	result := dfv1.VertexLimits{}
	if vLimits != nil {
		result.BufferMaxLength = vLimits.BufferMaxLength
		result.BufferUsageLimit = vLimits.BufferUsageLimit
		result.ReadBatchSize = vLimits.ReadBatchSize
		result.ReadTimeout = vLimits.ReadTimeout
	}
	if result.ReadBatchSize == nil {
		result.ReadBatchSize = plLimits.ReadBatchSize
	}
	if result.ReadTimeout == nil {
		result.ReadTimeout = plLimits.ReadTimeout
	}
	if result.BufferMaxLength == nil {
		result.BufferMaxLength = plLimits.BufferMaxLength
	}
	if result.BufferUsageLimit == nil {
		result.BufferUsageLimit = plLimits.BufferUsageLimit
	}
	return result
}

// Copy everything defined in the vertex template to the vertex object
// Be aware Maps will be merge copy, slices will be ignored if the destination is not empty
func copyVertexTemplate(pl *dfv1.Pipeline, vtx *dfv1.AbstractVertex) {
	if pl.Spec.Templates == nil || pl.Spec.Templates.VertexTemplate == nil {
		return
	}
	_ = mergo.Merge(&vtx.AbstractPodTemplate, pl.Spec.Templates.VertexTemplate.AbstractPodTemplate)

	if pl.Spec.Templates.VertexTemplate.ContainerTemplate != nil {
		if vtx.ContainerTemplate == nil {
			vtx.ContainerTemplate = &dfv1.ContainerTemplate{}
		}
		_ = mergo.Merge(vtx.ContainerTemplate, *pl.Spec.Templates.VertexTemplate.ContainerTemplate)
	}

	if pl.Spec.Templates.VertexTemplate.InitContainerTemplate != nil {
		if vtx.InitContainerTemplate == nil {
			vtx.InitContainerTemplate = &dfv1.ContainerTemplate{}
		}
		_ = mergo.Merge(vtx.InitContainerTemplate, *pl.Spec.Templates.VertexTemplate.InitContainerTemplate)
	}
}

func copyEdges(pl *dfv1.Pipeline, edges []dfv1.Edge) []dfv1.CombinedEdge {
	result := []dfv1.CombinedEdge{}
	for _, e := range edges {
		vFrom := pl.GetVertex(e.From)
		vTo := pl.GetVertex(e.To)
		fromVertexLimits := mergeLimits(pl.GetPipelineLimits(), vFrom.Limits)
		toVertexLimits := mergeLimits(pl.GetPipelineLimits(), vTo.Limits)
		combinedEdge := dfv1.CombinedEdge{
			Edge:                     e,
			FromVertexType:           vFrom.GetVertexType(),
			FromVertexPartitionCount: ptr.To[int32](int32(vFrom.GetPartitionCount())),
			FromVertexLimits:         &fromVertexLimits,
			ToVertexLimits:           &toVertexLimits,
			ToVertexType:             vTo.GetVertexType(),
			ToVertexPartitionCount:   ptr.To[int32](int32(vTo.GetPartitionCount())),
		}
		result = append(result, combinedEdge)
	}
	return result
}

func buildISBBatchJob(pl *dfv1.Pipeline, image string, isbSvcConfig dfv1.BufferServiceConfig, subCommand string, args []string, jobType string) *batchv1.Job {
	isbsType, envs := sharedutil.GetIsbSvcEnvVars(isbSvcConfig)
	envs = append(envs, corev1.EnvVar{Name: dfv1.EnvPipelineName, Value: pl.Name})
	c := corev1.Container{
		Name:            dfv1.CtrMain,
		Image:           image,
		ImagePullPolicy: corev1.PullPolicy(sharedutil.LookupEnvStringOr(dfv1.EnvImagePullPolicy, "")),
		Env:             envs,
	}
	c.Args = []string{subCommand, "--isbsvc-type=" + string(isbsType)}
	c.Args = append(c.Args, args...)
	randomStr := sharedutil.MustHash(pl.Spec)
	if len(randomStr) > 6 {
		randomStr = strings.ToLower(randomStr[:6])
	}
	l := map[string]string{
		dfv1.KeyPartOf:       dfv1.Project,
		dfv1.KeyManagedBy:    dfv1.ControllerPipeline,
		dfv1.KeyComponent:    dfv1.ComponentJob,
		dfv1.KeyPipelineName: pl.Name,
	}
	spec := batchv1.JobSpec{
		TTLSecondsAfterFinished: ptr.To[int32](30),
		BackoffLimit:            ptr.To[int32](20),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      l,
				Annotations: map[string]string{},
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyOnFailure,
				Containers:    []corev1.Container{c},
			},
		},
	}
	if pl.Spec.Templates != nil && pl.Spec.Templates.JobTemplate != nil {
		jt := pl.Spec.Templates.JobTemplate
		if jt.TTLSecondsAfterFinished != nil {
			spec.TTLSecondsAfterFinished = jt.TTLSecondsAfterFinished
		}
		if jt.BackoffLimit != nil {
			spec.BackoffLimit = jt.BackoffLimit
		}
		jt.AbstractPodTemplate.ApplyToPodTemplateSpec(&spec.Template)
		if jt.ContainerTemplate != nil {
			jt.ContainerTemplate.ApplyToNumaflowContainers(spec.Template.Spec.Containers)
		}
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pl.Namespace,
			// The name won't be over length limit, because we have validated "{pipeline}-{vertex}-headless" is no longer than 63.
			Name:   fmt.Sprintf("%s-%s-%v", pl.Name, jobType, randomStr),
			Labels: l,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pl.GetObjectMeta(), dfv1.PipelineGroupVersionKind),
			},
		},
		Spec: spec,
	}
}

type vertexFilterFunc func(v dfv1.Vertex) bool

var allVertexFilter vertexFilterFunc = func(v dfv1.Vertex) bool { return true }
var sourceVertexFilter vertexFilterFunc = func(v dfv1.Vertex) bool { return v.IsASource() }

func (r *pipelineReconciler) updateDesiredState(ctx context.Context, pl *dfv1.Pipeline) (bool, error) {
	switch pl.Spec.Lifecycle.GetDesiredPhase() {
	case dfv1.PipelinePhasePaused:
		return r.pausePipeline(ctx, pl)
	case dfv1.PipelinePhaseRunning, dfv1.PipelinePhaseUnknown:
		return r.resumePipeline(ctx, pl)
	default:
		return false, fmt.Errorf("invalid desired phase")
	}
}

func (r *pipelineReconciler) resumePipeline(ctx context.Context, pl *dfv1.Pipeline) (bool, error) {
	// reset pause timestamp
	if pl.GetAnnotations()[dfv1.KeyPauseTimestamp] != "" {
		err := r.client.Patch(ctx, pl, client.RawPatch(types.JSONPatchType, []byte(`[{"op": "remove", "path": "`+pauseTimestampPath+`"}]`)))
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil // skip pipeline if it can't be found
			} else {
				return true, err // otherwise requeue resume
			}
		}
	}
	_, err := r.scaleUpAllVertices(ctx, pl)
	if err != nil {
		return false, err
	}
	// mark the drained field as false to refresh the drained status as this will
	// be a new lifecycle from running
	pl.Status.MarkDrainedOnPauseFalse()
	pl.Status.MarkPhaseRunning()
	return false, nil
}

func (r *pipelineReconciler) pausePipeline(ctx context.Context, pl *dfv1.Pipeline) (bool, error) {
	// check that annotations / pause timestamp annotation exist
	if pl.GetAnnotations() == nil || pl.GetAnnotations()[dfv1.KeyPauseTimestamp] == "" {
		patchJson := `{"metadata":{"annotations":{"` + dfv1.KeyPauseTimestamp + `":"` + time.Now().Format(time.RFC3339) + `"}}}`
		if err := r.client.Patch(ctx, pl, client.RawPatch(types.MergePatchType, []byte(patchJson))); err != nil && !apierrors.IsNotFound(err) {
			return true, err
		}
	}

	pl.Status.MarkPhasePausing()
	updated, err := r.scaleDownSourceVertices(ctx, pl)
	if err != nil || updated {
		// If there's an error, or scaling down happens, requeue the request
		// This is to give some time to process the new messages, otherwise check IsDrained directly may get incorrect information
		return updated, err
	}

	var daemonError error
	var drainCompleted = false

	// Check for the daemon to obtain the buffer draining information, in case we see an error trying to
	// retrieve this we do not exit prematurely to allow honoring the pause timeout for a consistent error
	// - In case the timeout has not occurred we would trigger a requeue
	// - If the timeout has occurred even after getting the drained error, we will try to pause the pipeline
	daemonClient, daemonError := daemonclient.NewGRPCDaemonServiceClient(pl.GetDaemonServiceURL())
	r.logger.Info(daemonClient.ListPipelineBuffers(ctx, pl.Name))
	if err != nil {
		daemonError = err
	}
	if daemonClient != nil {
		defer func() {
			_ = daemonClient.Close()
		}()
		drainCompleted, err = daemonClient.IsDrained(ctx, pl.Name)
		if err != nil {
			daemonError = err
		}
	}
	pauseTimestamp, err := time.Parse(time.RFC3339, pl.GetAnnotations()[dfv1.KeyPauseTimestamp])
	if err != nil {
		return false, err
	}

	// if drain is completed, or we have exceeded the pause deadline, mark pl as paused and scale down
	if time.Now().After(pauseTimestamp.Add(time.Duration(pl.Spec.Lifecycle.GetPauseGracePeriodSeconds())*time.Second)) || drainCompleted {
		_, err = r.scaleDownAllVertices(ctx, pl)
		if err != nil {
			return true, err
		}
		if daemonError != nil {
			r.logger.Errorf("error in fetching Drained status, Pausing due to timeout: %v", zap.Error(err))
		}
		// if the drain completed successfully, then set the DrainedOnPause field to true
		if drainCompleted {
			pl.Status.MarkDrainedOnPauseTrue()
		}
		pl.Status.MarkPhasePaused()
		return false, nil
	}
	return true, daemonError
}

func (r *pipelineReconciler) scaleDownSourceVertices(ctx context.Context, pl *dfv1.Pipeline) (bool, error) {
	return r.scaleVertex(ctx, pl, sourceVertexFilter, 0)
}

func (r *pipelineReconciler) scaleDownAllVertices(ctx context.Context, pl *dfv1.Pipeline) (bool, error) {
	return r.scaleVertex(ctx, pl, allVertexFilter, 0)
}

func (r *pipelineReconciler) scaleUpAllVertices(ctx context.Context, pl *dfv1.Pipeline) (bool, error) {
	return r.scaleVertex(ctx, pl, allVertexFilter, 1)
}

func (r *pipelineReconciler) scaleVertex(ctx context.Context, pl *dfv1.Pipeline, filter vertexFilterFunc, replicas int32) (bool, error) {
	log := logging.FromContext(ctx)
	existingVertices, err := r.findExistingVertices(ctx, pl)
	if err != nil {
		return false, err
	}
	isVertexPatched := false
	for _, vertex := range existingVertices {
		if origin := *vertex.Spec.Replicas; origin != replicas && filter(vertex) {
			scaleTo := replicas
			// if replicas equals to 1, it means we are resuming a paused pipeline
			// in this case, if a vertex doesn't support auto-scaling, we scale up based on the vertex's configuration:
			// for a reducer, we scale up to the partition count
			// for a non-reducer, if min is set, we scale up to min
			if replicas == 1 {
				if !vertex.Scalable() {
					if vertex.IsReduceUDF() {
						scaleTo = int32(vertex.GetPartitionCount())
					} else if vertex.Spec.Scale.Min != nil && *vertex.Spec.Scale.Min > 1 {
						scaleTo = *vertex.Spec.Scale.Min
					}
				}
			}
			patchJson := fmt.Sprintf(`{"spec":{"replicas":%d}}`, scaleTo)
			err = r.client.Patch(ctx, &vertex, client.RawPatch(types.MergePatchType, []byte(patchJson)))
			if err != nil && !apierrors.IsNotFound(err) {
				return false, err
			}
			log.Infow("Scaled vertex", zap.Int32("from", origin), zap.Int32("to", scaleTo), zap.String("vertex", vertex.Name))
			r.recorder.Eventf(pl, corev1.EventTypeNormal, "ScalingVertex", "Scaled vertex %s from %d to %d replicas", vertex.Name, origin, scaleTo)
			isVertexPatched = true
		}
	}
	return isVertexPatched, nil
}

func (r *pipelineReconciler) safeToDelete(ctx context.Context, pl *dfv1.Pipeline) (bool, error) {
	// update the phase to deleting
	pl.Status.MarkPhaseDeleting()
	vertexPatched, err := r.scaleDownSourceVertices(ctx, pl)
	if err != nil {
		return false, err
	}
	// Requeue pipeline to take effect the vertex replica changes
	if vertexPatched {
		return false, nil
	}
	daemonClient, err := daemonclient.NewGRPCDaemonServiceClient(pl.GetDaemonServiceURL())
	if err != nil {
		return false, err
	}
	defer func() {
		_ = daemonClient.Close()
	}()
	return daemonClient.IsDrained(ctx, pl.Name)
}

// checkChildrenResourceStatus checks the status of the children resources of the pipeline
func (r *pipelineReconciler) checkChildrenResourceStatus(ctx context.Context, pipeline *dfv1.Pipeline) error {
	defer func() {
		for _, c := range pipeline.Status.Conditions {
			if c.Status != metav1.ConditionTrue {
				pipeline.Status.Message = "Degraded: " + c.Message
				return
			}
		}
	}()

	// get the daemon deployment and update the status of it to the pipeline
	var daemonDeployment appv1.Deployment
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: pipeline.GetNamespace(), Name: pipeline.GetDaemonDeploymentName()},
		&daemonDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			pipeline.Status.MarkDaemonServiceUnHealthy(
				"GetDaemonServiceFailed", "Deployment not found, might be still under creation")
			return nil
		}
		pipeline.Status.MarkDaemonServiceUnHealthy("GetDaemonServiceFailed", err.Error())
		return err
	}
	if status, reason, msg := reconciler.CheckDeploymentStatus(&daemonDeployment); status {
		pipeline.Status.MarkDaemonServiceHealthy()
	} else {
		pipeline.Status.MarkDaemonServiceUnHealthy(reason, msg)
	}

	// get the side input deployments and update the status of them to the pipeline
	if len(pipeline.Spec.SideInputs) == 0 {
		pipeline.Status.MarkSideInputsManagersHealthyWithReason(
			"NoSideInputs", "No Side Inputs attached to the pipeline")
	} else {
		var sideInputs appv1.DeploymentList
		selector, _ := labels.Parse(dfv1.KeyPipelineName + "=" + pipeline.Name + "," + dfv1.KeyComponent + "=" + dfv1.ComponentSideInputManager)
		if err := r.client.List(ctx, &sideInputs, &client.ListOptions{Namespace: pipeline.Namespace, LabelSelector: selector}); err != nil {
			pipeline.Status.MarkSideInputsManagersUnHealthy("ListSideInputsManagersFailed", err.Error())
			return err
		}
		for _, sideInput := range sideInputs.Items {
			if status, reason, msg := reconciler.CheckDeploymentStatus(&sideInput); status {
				pipeline.Status.MarkSideInputsManagersHealthy()
			} else {
				pipeline.Status.MarkSideInputsManagersUnHealthy(reason, msg)
				break
			}
		}
	}

	// calculate the status of the vertices and update the status of them to the pipeline
	var vertices dfv1.VertexList
	selector, _ := labels.Parse(dfv1.KeyPipelineName + "=" + pipeline.GetName() + "," + dfv1.KeyComponent + "=" + dfv1.ComponentVertex)
	if err := r.client.List(ctx, &vertices, &client.ListOptions{Namespace: pipeline.Namespace, LabelSelector: selector}); err != nil {
		pipeline.Status.MarkVerticesUnHealthy("ListVerticesFailed", err.Error())
		return err
	}
	status, reason, message := reconciler.CheckVertexStatus(&vertices)
	if status {
		pipeline.Status.MarkVerticesHealthy()
	} else {
		pipeline.Status.MarkVerticesUnHealthy(reason, "Some Vertices are unhealthy: "+message)
	}

	return nil
}
