/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	devboxv1alpha2 "github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	"github.com/sealos-apps/devbox/v2/controller/internal/controller/helper"
	"github.com/sealos-apps/devbox/v2/controller/internal/controller/utils/matcher"
	"github.com/sealos-apps/devbox/v2/controller/internal/controller/utils/resource"
	"github.com/sealos-apps/devbox/v2/controller/internal/controller/utils/rwords"
	"github.com/sealos-apps/devbox/v2/controller/label"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DevboxReconciler reconciles a Devbox object
type DevboxReconciler struct {
	CommitImageRegistry string
	DevboxNodeLabel     string

	RequestRate      resource.RequestRate
	EphemeralStorage resource.EphemeralStorage

	PodMatchers []matcher.PodMatcher

	DebugMode                 bool
	EnableBlockIOResource     bool
	StartupConfigMapName      string
	StartupConfigMapNamespace string

	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	RestartPredicateDuration time.Duration
}

// +kubebuilder:rbac:groups=devbox.sealos.io,resources=devboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devbox.sealos.io,resources=devboxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=devbox.sealos.io,resources=devboxes/finalizers,verbs=update
// +kubebuilder:rbac:groups=node.k8s.io,resources=runtimeclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=*
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=*
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=*
// +kubebuilder:rbac:groups="",resources=secrets,verbs=*
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=*
// +kubebuilder:rbac:groups="",resources=events,verbs=*
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=*
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=*

func (r *DevboxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("devbox", req.NamespacedName)

	// 1) Fetch the object. If it's gone, nothing to do.
	devbox := &devboxv1alpha2.Devbox{}
	if err := r.Get(ctx, req.NamespacedName, devbox); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	recLabels := label.RecommendedLabels(&label.Recommended{
		Name:      devbox.Name,
		ManagedBy: label.DefaultManagedBy,
		PartOf:    devboxv1alpha2.LabelDevBoxPartOf,
	})

	logger.Info("start reconciling devbox")
	if r.StartupConfigMapName != "" {
		logger.Info(
			"startup config map set",
			"startupConfigMapName", r.StartupConfigMapName,
			"startupConfigMapNamespace", r.StartupConfigMapNamespace,
		)
	}

	// 2) Deletion flow: make best-effort to delete sub-resources, then remove
	// the finalizer after node-local content is cleaned up.
	if !devbox.DeletionTimestamp.IsZero() {
		return r.reconcileDevboxDeletion(ctx, devbox, recLabels)
	}

	// 3) Ensure finalizer exists (idempotent).
	if err := r.ensureDevboxFinalizer(ctx, req.NamespacedName); err != nil {
		return ctrl.Result{}, err
	}

	// 4) Initialize status (idempotent). If we updated status, requeue to continue with the persisted status.
	updated, err := r.initDevboxStatus(ctx, devbox)
	if err != nil {
		return ctrl.Result{}, err
	}
	if updated {
		return ctrl.Result{Requeue: true}, nil
	}

	// 5) Validate required status fields for the rest of the flow.
	commitRecord, requeue, err := r.getCurrentCommitRecord(devbox)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		logger.Info("commit record is not found, requeue to wait for commit record to be created")
		return ctrl.Result{Requeue: true}, nil
	}

	// 6) Observe kube-scheduler's pod binding and persist the actual node.
	// The controller no longer pre-claims a node before creating the Pod; GPU and
	// other extended resources are scheduled by Kubernetes.
	res, err := r.syncAssignedNodeFromPod(ctx, req.NamespacedName, devbox, recLabels)
	if err != nil {
		return ctrl.Result{}, err
	}
	if res.Requeue || res.RequeueAfter > 0 {
		return res, nil
	}
	if err := r.Get(ctx, req.NamespacedName, devbox); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	commitRecord, requeue, err = r.getCurrentCommitRecord(devbox)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		logger.Info("commit record is not found after syncing assigned node, requeue")
		return ctrl.Result{Requeue: true}, nil
	}

	// 7) While the Pod is still waiting for kube-scheduler, only maintain the Pod
	// needed for scheduling. Once bound, this global controller runs the full
	// Kubernetes resource pipeline; node-local workers only handle runtime content.
	ownerNode := r.devboxOwnerNode(devbox, commitRecord)
	if devbox.Spec.State == devboxv1alpha2.DevboxStateRunning && ownerNode == "" {
		if err := r.syncSecret(ctx, devbox, recLabels); err != nil {
			return ctrl.Result{}, err
		}
		if r.StartupConfigMapName != "" {
			if err := r.syncStartupConfigMap(ctx, devbox, recLabels); err != nil {
				return ctrl.Result{}, err
			}
		}
		if isKubeAccessEnabled(devbox) {
			if err := r.syncKubeAccess(ctx, devbox, recLabels); err != nil {
				return ctrl.Result{}, err
			}
		}
		if err := r.syncPod(ctx, devbox, recLabels); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.syncDevboxPhase(ctx, devbox, recLabels); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.syncPodReadyCondition(ctx, devbox, recLabels); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 8) Reconcile desired Kubernetes resources (pods/services/secrets/etc).
	if err := r.runSyncPipeline(ctx, devbox, recLabels); err != nil {
		return ctrl.Result{}, err
	}

	// 9) Sync state transitions that do not require node-local content commit.
	if err := r.syncDevboxStateTransition(ctx, devbox); err != nil {
		return ctrl.Result{}, err
	}

	// 10) Keep conditions/ObservedGeneration in sync.
	if err := r.syncDevboxConditions(ctx, devbox); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("devbox reconcile success")
	return ctrl.Result{}, nil
}

func (r *DevboxReconciler) initDevboxStatus(
	ctx context.Context,
	devbox *devboxv1alpha2.Devbox,
) (updated bool, err error) {
	// only fill missing fields; avoid overriding existing status
	changed := false

	// init devbox status network type
	if devbox.Status.Network.Type == "" {
		devbox.Status.Network.Type = devbox.Spec.NetworkSpec.Type
		changed = true
	}

	// init devbox status content id
	if devbox.Status.ContentID == "" {
		devbox.Status.ContentID = uuid.New().String()
		changed = true
	}
	currentContentID := devbox.Status.ContentID

	// init devbox status commit record map
	if devbox.Status.CommitRecords == nil {
		devbox.Status.CommitRecords = make(map[string]*devboxv1alpha2.CommitRecord)
		changed = true
	}

	// init devbox status commit record for current content id
	if devbox.Status.CommitRecords[currentContentID] == nil {
		runtimeClassName := helper.ResolveRuntimeClassName(devbox.Spec.RuntimeClassName)
		devbox.Status.CommitRecords[currentContentID] = &devboxv1alpha2.CommitRecord{
			Node:             "",
			BaseImage:        devbox.Spec.Image,
			CommitImage:      r.generateImageName(devbox),
			CommitStatus:     devboxv1alpha2.CommitStatusPending,
			GenerateTime:     metav1.Now(),
			RuntimeClassName: runtimeClassName,
		}
		if _, err := helper.EnsureCommitRecordRuntimeMetadata(
			ctx,
			r.Client,
			devbox.Status.CommitRecords[currentContentID],
			runtimeClassName,
		); err != nil {
			return false, err
		}
		changed = true
	}

	if metadataChanged, err := helper.EnsureCommitRecordRuntimeMetadata(
		ctx,
		r.Client,
		devbox.Status.CommitRecords[currentContentID],
		devbox.Spec.RuntimeClassName,
	); err != nil {
		return false, err
	} else if metadataChanged {
		changed = true
	}

	// init devbox status state
	if devbox.Status.State == "" {
		devbox.Status.State = devbox.Spec.State
		changed = true
	}

	// init devbox status network unique id
	if devbox.Status.Network.UniqueID == "" {
		devbox.Status.Network.UniqueID = rwords.GenerateRandomWords()
		changed = true
	}

	// update devbox status, and do not return error to avoid infinite loop because multiple controller will reconcile this devbox
	if changed {
		if err := r.Status().Update(ctx, devbox); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

// reconcileDevboxDeletion deletes owned resources then removes the devbox finalizer.
func (r *DevboxReconciler) reconcileDevboxDeletion(
	ctx context.Context,
	devbox *devboxv1alpha2.Devbox,
	recLabels map[string]string,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("devbox", client.ObjectKeyFromObject(devbox))

	logger.Info("devbox deleted, remove all resources")
	if err := r.handleSubResourceDelete(ctx, devbox, recLabels); err != nil {
		return ctrl.Result{}, err
	}

	cleanupNode := r.localStorageCleanupNode(devbox)
	if cleanupNode != "" {
		logger.Info(
			"devbox deletion is waiting for node-local storage cleanup",
			"node",
			cleanupNode,
		)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	logger.Info("devbox deleted, remove finalizer")
	if controllerutil.RemoveFinalizer(devbox, devboxv1alpha2.FinalizerName) {
		if err := r.Update(ctx, devbox); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// ensureDevboxFinalizer ensures the devbox has the controller finalizer (idempotent).
func (r *DevboxReconciler) ensureDevboxFinalizer(ctx context.Context, key client.ObjectKey) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &devboxv1alpha2.Devbox{}
		if err := r.Get(ctx, key, latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if !controllerutil.ContainsFinalizer(latest, devboxv1alpha2.FinalizerName) {
			controllerutil.AddFinalizer(latest, devboxv1alpha2.FinalizerName)
			return r.Update(ctx, latest)
		}
		return nil
	})
}

// getCurrentCommitRecord returns the current commit record.
// If it is missing, caller should requeue (not an error).
func (r *DevboxReconciler) getCurrentCommitRecord(
	devbox *devboxv1alpha2.Devbox,
) (record *devboxv1alpha2.CommitRecord, requeue bool, err error) {
	if devbox.Status.ContentID == "" {
		return nil, true, nil
	}
	if devbox.Status.CommitRecords == nil {
		return nil, true, nil
	}
	rec := devbox.Status.CommitRecords[devbox.Status.ContentID]
	if rec == nil {
		return nil, true, nil
	}
	return rec, false, nil
}

func (r *DevboxReconciler) localStorageCleanupNode(devbox *devboxv1alpha2.Devbox) string {
	if devbox == nil {
		return ""
	}
	if devbox.Status.State == devboxv1alpha2.DevboxStateStopped ||
		devbox.Status.State == devboxv1alpha2.DevboxStateShutdown {
		return ""
	}
	record := helper.GetLatestCommitRecord(devbox.Status.CommitRecords, devbox.Status.ContentID)
	if record == nil {
		return ""
	}
	return record.Node
}

func (r *DevboxReconciler) devboxOwnerNode(
	_ *devboxv1alpha2.Devbox,
	commitRecord *devboxv1alpha2.CommitRecord,
) string {
	if commitRecord != nil {
		return commitRecord.Node
	}
	return ""
}

// syncAssignedNodeFromPod mirrors the kube-scheduler assignment into Devbox status.
// It returns a requeue result when status changed so the next reconcile uses fresh
// ownership data before touching local-runtime resources.
func (r *DevboxReconciler) syncAssignedNodeFromPod(
	ctx context.Context,
	key client.ObjectKey,
	devbox *devboxv1alpha2.Devbox,
	recLabels map[string]string,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("devbox", key)

	podList := &corev1.PodList{}
	if err := r.List(
		ctx,
		podList,
		client.InNamespace(devbox.Namespace),
		client.MatchingLabels(recLabels),
	); err != nil {
		return ctrl.Result{}, err
	}
	if len(podList.Items) != 1 {
		return ctrl.Result{}, nil
	}

	pod := &podList.Items[0]
	assignedNode := pod.Spec.NodeName
	if assignedNode == "" || !pod.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	podContentID := ""
	if pod.Annotations != nil {
		podContentID = pod.Annotations[devboxv1alpha2.AnnotationContentID]
	}
	if podContentID != "" && podContentID != devbox.Status.ContentID {
		logger.Info(
			"pod content id does not match current devbox content, skip node sync",
			"pod",
			pod.Name,
			"podContentID",
			podContentID,
			"contentID",
			devbox.Status.ContentID,
		)
		return ctrl.Result{}, nil
	}

	updated := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &devboxv1alpha2.Devbox{}
		if err := r.Get(ctx, key, latest); err != nil {
			return err
		}
		if latest.Status.CommitRecords == nil ||
			latest.Status.CommitRecords[latest.Status.ContentID] == nil {
			return fmt.Errorf("commit record missing for contentID %s", latest.Status.ContentID)
		}

		latestRecord := latest.Status.CommitRecords[latest.Status.ContentID]
		ownerNode := latestRecord.Node
		if ownerNode == "" {
			ownerNode = latest.Status.Node
		}
		if ownerNode != "" && ownerNode != assignedNode {
			logger.Info(
				"pod was assigned to a different node than the current content owner, deleting pod",
				"pod",
				pod.Name,
				"ownerNode",
				ownerNode,
				"assignedNode",
				assignedNode,
			)
			r.Recorder.Eventf(
				devbox,
				corev1.EventTypeWarning,
				"Devbox node ownership mismatch",
				"Pod assigned to node %s but current content is owned by node %s",
				assignedNode,
				ownerNode,
			)
			if err := r.deleteUnexpectedAssignedPod(ctx, pod); err != nil {
				return err
			}
			return nil
		}

		if latest.Status.Node == assignedNode && latestRecord.Node == assignedNode {
			return nil
		}

		latestRecord.Node = assignedNode
		latestRecord.ScheduleTime = metav1.Now()
		latest.Status.Node = assignedNode
		if err := r.Status().Update(ctx, latest); err != nil {
			return err
		}
		updated = true
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	if updated {
		logger.Info("devbox assigned by kube-scheduler", "node", assignedNode)
		r.Recorder.Eventf(
			devbox,
			corev1.EventTypeNormal,
			"Devbox scheduled to node",
			"Devbox scheduled to node %s",
			assignedNode,
		)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *DevboxReconciler) deleteUnexpectedAssignedPod(ctx context.Context, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)
	originalPodUID := pod.UID

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestPod := &corev1.Pod{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(pod), latestPod); err != nil {
			return client.IgnoreNotFound(err)
		}
		if latestPod.UID != originalPodUID {
			logger.Info(
				"pod UID changed, skip unexpected pod deletion",
				"pod",
				pod.Name,
				"originalUID",
				originalPodUID,
				"currentUID",
				latestPod.UID,
			)
			return nil
		}
		if controllerutil.RemoveFinalizer(latestPod, devboxv1alpha2.FinalizerName) {
			return r.Update(ctx, latestPod)
		}
		return nil
	}); err != nil {
		return err
	}

	latestPod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(pod), latestPod); err != nil {
		return client.IgnoreNotFound(err)
	}
	if latestPod.UID != originalPodUID {
		logger.Info(
			"pod UID changed, skip unexpected pod deletion",
			"pod",
			pod.Name,
			"originalUID",
			originalPodUID,
			"currentUID",
			latestPod.UID,
		)
		return nil
	}
	if err := r.Delete(
		ctx,
		latestPod,
		client.GracePeriodSeconds(0),
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

func (r *DevboxReconciler) syncDevboxStateTransition(
	ctx context.Context,
	devbox *devboxv1alpha2.Devbox,
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &devboxv1alpha2.Devbox{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(devbox), latest); err != nil {
			return err
		}

		if latest.Spec.State == latest.Status.State {
			return nil
		}

		currentRecord := helper.GetLatestCommitRecord(
			latest.Status.CommitRecords,
			latest.Status.ContentID,
		)
		if devboxStateTransitionNeedsLocalCommit(latest, currentRecord) {
			latest.SetCondition(metav1.Condition{
				Type:               devboxv1alpha2.DevboxConditionStateTransitionPending,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: latest.Generation,
				Reason:             devboxv1alpha2.DevboxReasonSpecStateChanged,
				Message:            "waiting for node-local commit worker to persist content",
				LastTransitionTime: metav1.Now(),
			})
			return r.Status().Update(ctx, latest)
		}

		latest.Status.State = latest.Spec.State
		latest.Status.ObservedGeneration = latest.Generation
		latest.SetCondition(metav1.Condition{
			Type:               devboxv1alpha2.DevboxConditionStateTransitionPending,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: latest.Generation,
			Reason:             devboxv1alpha2.DevboxReasonStateTransitionSynced,
			Message:            "spec.state matches status.state",
			LastTransitionTime: metav1.Now(),
		})
		return r.Status().Update(ctx, latest)
	})
}

func devboxStateTransitionNeedsLocalCommit(
	devbox *devboxv1alpha2.Devbox,
	currentRecord *devboxv1alpha2.CommitRecord,
) bool {
	if devbox == nil || currentRecord == nil {
		return false
	}
	targetStopOrShutdown := devbox.Spec.State == devboxv1alpha2.DevboxStateStopped ||
		devbox.Spec.State == devboxv1alpha2.DevboxStateShutdown
	currentRunningOrPaused := devbox.Status.State == devboxv1alpha2.DevboxStateRunning ||
		devbox.Status.State == devboxv1alpha2.DevboxStatePaused
	return targetStopOrShutdown && currentRunningOrPaused && currentRecord.Node != ""
}

func (r *DevboxReconciler) syncDevboxConditions(
	ctx context.Context,
	devbox *devboxv1alpha2.Devbox,
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &devboxv1alpha2.Devbox{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(devbox), latest); err != nil {
			return err
		}

		// Only advance ObservedGeneration when the state transition is fully synced.
		// (Other spec changes are currently reconciled in the same flow; using state
		// as the hard gate avoids reporting a generation as "observed" while a
		// transition is still pending.)
		if latest.Spec.State == latest.Status.State {
			latest.Status.ObservedGeneration = latest.Generation
		}

		currentRecord := helper.GetLatestCommitRecord(
			latest.Status.CommitRecords,
			latest.Status.ContentID,
		)

		// State transition pending condition
		if latest.Spec.State != latest.Status.State {
			message := "spec.state differs from status.state; state transition pending"
			if devboxStateTransitionNeedsLocalCommit(latest, currentRecord) {
				message = "waiting for node-local commit worker to persist content"
			}
			latest.SetCondition(metav1.Condition{
				Type:               devboxv1alpha2.DevboxConditionStateTransitionPending,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: latest.Generation,
				Reason:             devboxv1alpha2.DevboxReasonSpecStateChanged,
				Message:            message,
				LastTransitionTime: metav1.Now(),
			})
		} else {
			latest.SetCondition(metav1.Condition{
				Type:               devboxv1alpha2.DevboxConditionStateTransitionPending,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: latest.Generation,
				Reason:             devboxv1alpha2.DevboxReasonStateTransitionSynced,
				Message:            "spec.state matches status.state",
				LastTransitionTime: metav1.Now(),
			})
		}

		return r.Status().Update(ctx, latest)
	})
}

func (r *DevboxReconciler) generateImageName(devbox *devboxv1alpha2.Devbox) string {
	now := time.Now()
	return fmt.Sprintf(
		"%s/%s/%s:%s-%s",
		r.CommitImageRegistry,
		devbox.Namespace,
		devbox.Name,
		rand.String(5),
		now.Format("2006-01-02-150405"),
	)
}

func (r *DevboxReconciler) handleSubResourceDelete(
	ctx context.Context,
	devbox *devboxv1alpha2.Devbox,
	recLabels map[string]string,
) error {
	logger := log.FromContext(ctx)

	// Delete Pod
	podList := &corev1.PodList{}
	if err := r.List(
		ctx,
		podList,
		client.InNamespace(devbox.Namespace),
		client.MatchingLabels(recLabels),
	); err != nil {
		return err
	}
	for i := range podList.Items {
		pod := &podList.Items[i]
		originalPodUID := pod.UID

		// Remove finalizer with retry and UID check
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latestPod := &corev1.Pod{}
			if err := r.Get(ctx, client.ObjectKeyFromObject(pod), latestPod); err != nil {
				if apierrors.IsNotFound(err) {
					// Pod already deleted
					logger.Info("pod already deleted, skip finalizer removal", "pod", pod.Name)
					return nil
				}
				return err
			}

			// Check if UID matches
			if latestPod.UID != originalPodUID {
				logger.Info("pod UID changed, skip finalizer removal",
					"pod", pod.Name,
					"originalUID", originalPodUID,
					"currentUID", latestPod.UID)
				return nil
			}

			if controllerutil.RemoveFinalizer(latestPod, devboxv1alpha2.FinalizerName) {
				return r.Update(ctx, latestPod)
			}
			return nil
		})
		if err != nil {
			logger.Error(err, "failed to remove finalizer from pod", "pod", pod.Name)
			return err
		}
	}
	if err := r.deleteResourcesByLabels(
		ctx,
		&corev1.Pod{},
		devbox.Namespace,
		recLabels,
	); err != nil {
		return err
	}
	// Delete Service
	if err := r.deleteResourcesByLabels(
		ctx,
		&corev1.Service{},
		devbox.Namespace,
		recLabels,
	); err != nil {
		return err
	}
	// Delete Configmap
	if err := r.deleteResourcesByLabels(
		ctx,
		&corev1.ConfigMap{},
		devbox.Namespace,
		recLabels,
	); err != nil {
		return err
	}
	// Delete Secret
	return r.deleteResourcesByLabels(ctx, &corev1.Secret{}, devbox.Namespace, recLabels)
}

func (r *DevboxReconciler) deleteResourcesByLabels(
	ctx context.Context,
	obj client.Object,
	namespace string,
	labels map[string]string,
) error {
	err := r.DeleteAllOf(ctx, obj,
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	)
	return client.IgnoreNotFound(err)
}

func (r *DevboxReconciler) setConditionWithRetry(
	ctx context.Context,
	devbox *devboxv1alpha2.Devbox,
	cond metav1.Condition,
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &devboxv1alpha2.Devbox{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(devbox), latest); err != nil {
			return err
		}
		cond.ObservedGeneration = latest.Generation
		cond.LastTransitionTime = metav1.Now()
		latest.SetCondition(cond)
		return r.Status().Update(ctx, latest)
	})
}

func (r *DevboxReconciler) setSyncCondition(
	ctx context.Context,
	devbox *devboxv1alpha2.Devbox,
	conditionType string,
	ok bool,
	message string,
) {
	logger := log.FromContext(ctx)
	status := metav1.ConditionFalse
	reason := devboxv1alpha2.DevboxReasonSyncFailed
	if ok {
		status = metav1.ConditionTrue
		reason = devboxv1alpha2.DevboxReasonSyncSucceeded
	}
	if err := r.setConditionWithRetry(ctx, devbox, metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}); err != nil {
		logger.Info(
			"failed to update condition (best-effort)",
			"conditionType",
			conditionType,
			"error",
			err,
		)
	}
}

// ContentIDChangedPredicate triggers reconcile when devbox status.contentID changes
type ContentIDChangedPredicate struct {
	predicate.Funcs
}

func (p ContentIDChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldDevbox, oldOk := e.ObjectOld.(*devboxv1alpha2.Devbox)
	newDevbox, newOk := e.ObjectNew.(*devboxv1alpha2.Devbox)
	if oldOk && newOk {
		return oldDevbox.Status.ContentID != newDevbox.Status.ContentID
	}

	return false
}

// LastContainerStatusChangedPredicate triggers reconcile when devbox status.lastContainerStatus changes
type LastContainerStatusChangedPredicate struct {
	predicate.Funcs
}

func (p LastContainerStatusChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldDevbox, oldOk := e.ObjectOld.(*devboxv1alpha2.Devbox)
	newDevbox, newOk := e.ObjectNew.(*devboxv1alpha2.Devbox)
	if oldOk && newOk {
		return oldDevbox.Status.LastContainerStatus.ContainerID != newDevbox.Status.LastContainerStatus.ContainerID
	}
	return false
}

// NetworkTypeChangedPredicate triggers reconcile when devbox status.network.type changes
type NetworkTypeChangedPredicate struct {
	predicate.Funcs
}

func (p NetworkTypeChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldDevbox, oldOk := e.ObjectOld.(*devboxv1alpha2.Devbox)
	newDevbox, newOk := e.ObjectNew.(*devboxv1alpha2.Devbox)
	if oldOk && newOk {
		return oldDevbox.Status.Network.Type != newDevbox.Status.Network.Type
	}
	return false
}

// StatusNodeChangedPredicate triggers reconcile when devbox status.node changes.
type StatusNodeChangedPredicate struct {
	predicate.Funcs
}

func (p StatusNodeChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldDevbox, oldOk := e.ObjectOld.(*devboxv1alpha2.Devbox)
	newDevbox, newOk := e.ObjectNew.(*devboxv1alpha2.Devbox)
	if oldOk && newOk {
		return oldDevbox.Status.Node != newDevbox.Status.Node
	}
	return false
}

// PhaseChangedPredicate triggers reconcile when devbox status.phase changes or status.phase is `Error`
type PhaseChangedPredicate struct {
	predicate.Funcs
}

func (p PhaseChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldDevbox, oldOk := e.ObjectOld.(*devboxv1alpha2.Devbox)
	newDevbox, newOk := e.ObjectNew.(*devboxv1alpha2.Devbox)
	if oldOk && newOk {
		return oldDevbox.Status.Phase != newDevbox.Status.Phase ||
			newDevbox.Status.Phase == devboxv1alpha2.DevboxPhaseError
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *DevboxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		For(&devboxv1alpha2.Devbox{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{}, // enqueue request if devbox spec is updated
			NetworkTypeChangedPredicate{},          // enqueue request if devbox status.network.type is updated
			StatusNodeChangedPredicate{},           // enqueue request if devbox status.node is updated
			ContentIDChangedPredicate{},            // enqueue request if devbox status.contentID is updated
			LastContainerStatusChangedPredicate{},  // enqueue request if devbox status.lastContainerStatus is updated
			PhaseChangedPredicate{},                // enqueue request if devbox status.phase is updated or status.phase is `Error`
		))).
		Owns(&corev1.Pod{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		// enqueue request if pod spec/status is updated
		Watches(
			&corev1.Event{},
			handler.EnqueueRequestsFromMapFunc(r.mapPodEventToDevbox),
			builder.WithPredicates(predicate.NewPredicateFuncs(isStorageFullPodEvent)),
		).
		// enqueue request if kubelet records a runtime event for a devbox pod
		Owns(&corev1.Service{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// enqueue request if service spec is updated
		Owns(&corev1.Secret{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

func (r *DevboxReconciler) mapPodEventToDevbox(
	_ context.Context,
	obj client.Object,
) []reconcile.Request {
	event, ok := obj.(*corev1.Event)
	if !ok {
		return nil
	}
	if event.InvolvedObject.Kind != "Pod" ||
		event.InvolvedObject.Name == "" ||
		event.InvolvedObject.Namespace == "" {
		return nil
	}
	return []reconcile.Request{{
		NamespacedName: client.ObjectKey{
			Namespace: event.InvolvedObject.Namespace,
			Name:      event.InvolvedObject.Name,
		},
	}}
}

func isStorageFullPodEvent(obj client.Object) bool {
	event, ok := obj.(*corev1.Event)
	if !ok {
		return false
	}
	return event.InvolvedObject.Kind == "Pod" &&
		event.InvolvedObject.Name != "" &&
		event.InvolvedObject.Namespace != "" &&
		isNoSpaceLeftOnDeviceMessage(event.Message)
}
