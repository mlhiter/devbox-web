package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	"github.com/sealos-apps/devbox/v2/controller/internal/controller/helper"
	"github.com/sealos-apps/devbox/v2/controller/label"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var errWaitingForDevboxPodDeletion = errors.New("waiting for devbox pod deletion")

// DevboxNodeReconciler runs node-local runtime work for Devbox content.
// It is intended for the commit DaemonSet: one worker per devbox node, with no
// leader election. The global DevboxReconciler owns Kubernetes resources and
// scheduling; this reconciler only commits or cleans up content on its node.
type DevboxNodeReconciler struct {
	Handler  *EventHandler
	NodeName string

	client.Client
}

func (r *DevboxNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("devbox", req.NamespacedName, "node", r.NodeName)

	devbox := &v1alpha2.Devbox{}
	if err := r.Get(ctx, req.NamespacedName, devbox); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	currentRecord := helper.GetLatestCommitRecord(
		devbox.Status.CommitRecords,
		devbox.Status.ContentID,
	)

	if !devbox.DeletionTimestamp.IsZero() {
		if currentRecord == nil || currentRecord.Node != r.NodeName {
			return ctrl.Result{}, nil
		}
		if devbox.Status.State == v1alpha2.DevboxStateStopped ||
			devbox.Status.State == v1alpha2.DevboxStateShutdown {
			return ctrl.Result{}, nil
		}
		if err := r.cleanupDeletedDevbox(ctx, devbox, currentRecord); err != nil {
			if errors.Is(err, errWaitingForDevboxPodDeletion) {
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}

	if !devboxStateTransitionNeedsLocalCommit(devbox, currentRecord) {
		return ctrl.Result{}, nil
	}
	if currentRecord.Node != r.NodeName {
		return ctrl.Result{}, nil
	}
	podsGone, err := r.devboxPodsGone(ctx, devbox)
	if err != nil {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}
	if !podsGone {
		logger.Info("waiting for devbox pod deletion before node-local commit")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if devbox.Status.ContentID == "" {
		err := errors.New("empty contentID, cannot start commit")
		logger.Error(err, "invalid devbox for node-local commit")
		return ctrl.Result{}, err
	}
	commitKey := devboxContentKey(devbox)
	if _, loaded := commitMap.LoadOrStore(commitKey, true); loaded {
		logger.Info("commit already in progress, skipping duplicate request", "commitKey", commitKey)
		return ctrl.Result{}, nil
	}
	defer commitMap.Delete(commitKey)

	logger.Info(
		"node-local commit required",
		"from",
		devbox.Status.State,
		"to",
		devbox.Spec.State,
		"contentID",
		devbox.Status.ContentID,
	)
	start := time.Now()
	err = retry.OnError(wait.Backoff{
		Duration: 10 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
		Steps:    30,
	}, func(err error) bool {
		return !errors.Is(err, context.Canceled) &&
			!errors.Is(err, context.DeadlineExceeded) &&
			!apierrors.IsNotFound(err)
	}, func() error {
		if err := r.Handler.commitDevbox(ctx, devbox, devbox.Spec.State); err != nil {
			logger.Error(err, "failed to commit devbox in retry")
			return err
		}
		return nil
	})
	if err != nil && apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}
	logger.Info("node-local commit finished", "duration", time.Since(start))
	return ctrl.Result{}, nil
}

func (r *DevboxNodeReconciler) cleanupDeletedDevbox(
	ctx context.Context,
	devbox *v1alpha2.Devbox,
	record *v1alpha2.CommitRecord,
) error {
	logger := log.FromContext(ctx).WithValues(
		"devbox",
		client.ObjectKeyFromObject(devbox),
		"contentID",
		devbox.Status.ContentID,
	)

	if record.BaseImage == "" {
		return fmt.Errorf("base image is empty for devbox %s contentID %s", devbox.Name, devbox.Status.ContentID)
	}
	podsGone, err := r.devboxPodsGone(ctx, devbox)
	if err != nil {
		return err
	}
	if !podsGone {
		logger.Info("waiting for devbox pod deletion before node-local storage cleanup")
		return errWaitingForDevboxPodDeletion
	}
	if _, err := helper.EnsureCommitRecordRuntimeMetadata(
		ctx,
		r.Client,
		record,
		devbox.Spec.RuntimeClassName,
	); err != nil {
		return fmt.Errorf("failed to resolve runtime metadata: %w", err)
	}

	deleteKey := devboxObjectKey(devbox)
	if _, loaded := deleteMap.LoadOrStore(deleteKey, true); loaded {
		logger.Info("storage cleanup already in progress, skipping duplicate request")
		return nil
	}
	defer deleteMap.Delete(deleteKey)

	err = retry.OnError(wait.Backoff{
		Duration: 10 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
		Steps:    30,
	}, func(error) bool {
		return true
	}, func() error {
		return r.Handler.cleanupStorage(
			ctx,
			devbox.Name,
			devbox.Status.ContentID,
			record.BaseImage,
			devbox.Spec.StorageLimit,
			record.Snapshotter,
			record.RuntimeClassName,
			record.RuntimeHandler,
		)
	})
	if err != nil {
		return err
	}

	logger.Info("node-local storage cleanup finished, removing devbox finalizer")
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &v1alpha2.Devbox{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(devbox), latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if controllerutil.RemoveFinalizer(latest, v1alpha2.FinalizerName) {
			return r.Update(ctx, latest)
		}
		return nil
	})
}

func devboxContentKey(devbox *v1alpha2.Devbox) string {
	return fmt.Sprintf("%s/%s/%s", devbox.Namespace, devbox.Name, devbox.Status.ContentID)
}

func devboxObjectKey(devbox *v1alpha2.Devbox) string {
	return fmt.Sprintf("%s/%s", devbox.Namespace, devbox.Name)
}

func (r *DevboxNodeReconciler) devboxPodsGone(
	ctx context.Context,
	devbox *v1alpha2.Devbox,
) (bool, error) {
	podList := &corev1.PodList{}
	if err := r.List(
		ctx,
		podList,
		client.InNamespace(devbox.Namespace),
		client.MatchingLabels(label.RecommendedLabels(&label.Recommended{
			Name:      devbox.Name,
			ManagedBy: label.DefaultManagedBy,
			PartOf:    v1alpha2.LabelDevBoxPartOf,
		})),
	); err != nil {
		return false, err
	}
	return len(podList.Items) == 0, nil
}

func (r *DevboxNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("devbox-node-worker").
		WithOptions(controller.Options{MaxConcurrentReconciles: 4}).
		For(&v1alpha2.Devbox{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{},
			ContentIDChangedPredicate{},
			StatusNodeChangedPredicate{},
			DevboxDeletionChangedPredicate{},
			DevboxStatusStateChangedPredicate{},
			CommitStatusChangedPredicate{},
		))).
		Complete(r)
}

type DevboxDeletionChangedPredicate struct {
	predicate.Funcs
}

func (p DevboxDeletionChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldDevbox, oldOk := e.ObjectOld.(*v1alpha2.Devbox)
	newDevbox, newOk := e.ObjectNew.(*v1alpha2.Devbox)
	if oldOk && newOk {
		return oldDevbox.DeletionTimestamp.IsZero() != newDevbox.DeletionTimestamp.IsZero()
	}
	return false
}

type DevboxStatusStateChangedPredicate struct {
	predicate.Funcs
}

func (p DevboxStatusStateChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldDevbox, oldOk := e.ObjectOld.(*v1alpha2.Devbox)
	newDevbox, newOk := e.ObjectNew.(*v1alpha2.Devbox)
	if oldOk && newOk {
		return oldDevbox.Status.State != newDevbox.Status.State
	}
	return false
}

type CommitStatusChangedPredicate struct {
	predicate.Funcs
}

func (p CommitStatusChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldDevbox, oldOk := e.ObjectOld.(*v1alpha2.Devbox)
	newDevbox, newOk := e.ObjectNew.(*v1alpha2.Devbox)
	if !oldOk || !newOk {
		return false
	}
	oldRecord := helper.GetLatestCommitRecord(oldDevbox.Status.CommitRecords, oldDevbox.Status.ContentID)
	newRecord := helper.GetLatestCommitRecord(newDevbox.Status.CommitRecords, newDevbox.Status.ContentID)
	if oldRecord == nil || newRecord == nil {
		return oldRecord != newRecord
	}
	return oldRecord.CommitStatus != newRecord.CommitStatus
}
