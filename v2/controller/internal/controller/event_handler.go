package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	devboxv1alpha2 "github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	"github.com/sealos-apps/devbox/v2/controller/internal/commit"
	"github.com/sealos-apps/devbox/v2/controller/internal/controller/helper"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	commitMap = sync.Map{}
	deleteMap = sync.Map{}
)

type EventHandler struct {
	Committers          map[string]commit.Committer
	CommitImageRegistry string
	DefaultBaseImage    string

	Logger logr.Logger
	Client client.Client
}

func (h *EventHandler) getCommitterBySnapshotter(snapshotter string) (commit.Committer, error) {
	selected := strings.TrimSpace(snapshotter)
	if selected == "" {
		selected = commit.DefaultDevboxSnapshotter
	}
	committer, ok := h.Committers[selected]
	if ok && committer != nil {
		return committer, nil
	}
	if fallback, ok := h.Committers[commit.DefaultDevboxSnapshotter]; ok && fallback != nil {
		h.Logger.Info(
			"snapshotter committer not found, fallback to devbox snapshotter",
			"requestedSnapshotter",
			selected,
			"fallbackSnapshotter",
			commit.DefaultDevboxSnapshotter,
		)
		return fallback, nil
	}
	return nil, fmt.Errorf("committer for snapshotter %q not found", selected)
}

func (h *EventHandler) commitDevbox(
	ctx context.Context,
	devbox *devboxv1alpha2.Devbox,
	targetState devboxv1alpha2.DevboxState,
) error {
	if err := h.Client.Get(
		ctx,
		types.NamespacedName{Namespace: devbox.Namespace, Name: devbox.Name},
		devbox,
	); err != nil {
		if apierrors.IsNotFound(err) {
			h.Logger.Info("devbox not found at start of commit", "devbox", devbox.Name)
			return err
		}
		h.Logger.Error(err, "failed to get devbox", "devbox", devbox.Name)
		return err
	}
	// do commit, update devbox commit record, update devbox status state to shutdown, add a new commit record for the new content id
	// step 0: set commit status to committing to prevent duplicate requests with retry
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestDevbox := &devboxv1alpha2.Devbox{}
		if err := h.Client.Get(
			ctx,
			types.NamespacedName{Namespace: devbox.Namespace, Name: devbox.Name},
			latestDevbox,
		); err != nil {
			// If devbox is not found, return the error to stop retrying
			if apierrors.IsNotFound(err) {
				return err
			}
			return err
		}
		currentRecord, err := getCurrentCommitRecord(latestDevbox)
		if err != nil {
			return err
		}
		currentRecord.CommitStatus = devboxv1alpha2.CommitStatusCommitting
		currentRecord.UpdateTime = metav1.Now()
		latestDevbox.SetCondition(metav1.Condition{
			Type:               devboxv1alpha2.DevboxConditionCommitInProgress,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: latestDevbox.Generation,
			Reason:             devboxv1alpha2.DevboxReasonCommitStarted,
			Message:            "commit workflow in progress",
			LastTransitionTime: metav1.Now(),
		})
		return h.Client.Status().Update(ctx, latestDevbox)
	}); err != nil {
		if apierrors.IsNotFound(err) {
			h.Logger.Info("devbox not found when setting commit status", "devbox", devbox.Name)
			return err
		}
		h.Logger.Error(err, "failed to update commit status to committing", "devbox", devbox.Name)
		return err
	}
	h.Logger.Info(
		"set commit status to committing",
		"devbox",
		devbox.Name,
		"contentID",
		devbox.Status.ContentID,
	)

	if err := h.Client.Get(
		ctx,
		types.NamespacedName{Namespace: devbox.Namespace, Name: devbox.Name},
		devbox,
	); err != nil {
		if apierrors.IsNotFound(err) {
			h.Logger.Info("devbox not found before commit", "devbox", devbox.Name)
			return err
		}
		h.Logger.Error(err, "failed to get devbox", "devbox", devbox.Name)
		return err
	}
	// step 1: do commit, push image, remove container whether commit success or not
	currentRecord, err := getCurrentCommitRecord(devbox)
	if err != nil {
		h.Logger.Error(err, "failed to get current commit record", "devbox", devbox.Name)
		return err
	}
	if _, err := helper.EnsureCommitRecordRuntimeMetadata(
		ctx,
		h.Client,
		currentRecord,
		devbox.Spec.RuntimeClassName,
	); err != nil {
		h.Logger.Error(err, "failed to resolve runtime metadata", "devbox", devbox.Name)
		return err
	}
	committer, err := h.getCommitterBySnapshotter(currentRecord.Snapshotter)
	if err != nil {
		h.Logger.Error(
			err,
			"failed to select committer by snapshotter",
			"devbox",
			devbox.Name,
			"snapshotter",
			currentRecord.Snapshotter,
		)
		return err
	}
	baseImage := currentRecord.BaseImage
	commitImage := currentRecord.CommitImage
	oldContentID := devbox.Status.ContentID
	h.Logger.Info(
		"commit devbox",
		"devbox",
		devbox.Name,
		"baseImage",
		baseImage,
		"commitImage",
		commitImage,
	)
	var containerID string
	var commitErr error
	removeImageNames := make([]string, 0, 1)
	defer func() {
		// remove container whether commit success or not
		if strings.TrimSpace(containerID) != "" {
			if err := committer.RemoveContainers(ctx, []string{containerID}); err != nil {
				h.Logger.Error(err, "failed to remove container", "containerID", containerID)
			}
		}
		// remove after push image whether push success
		if len(removeImageNames) > 0 {
			if err := committer.RemoveImages(
				ctx,
				removeImageNames,
				commit.DefaultRemoveImageForce,
				commit.DefaultRemoveImageAsync,
			); err != nil {
				if isImageInUseConflict(err) {
					h.Logger.Info(
						"skip removing image still referenced by container",
						"removeImageNames",
						removeImageNames,
						"error",
						err.Error(),
					)
				} else {
					h.Logger.Error(err, "failed to remove image", "removeImageNames", removeImageNames)
				}
			}
		}
	}()
	previousContainerID := normalizeContainerRuntimeID(devbox.Status.LastContainerStatus.ContainerID)
	if previousContainerID != "" {
		h.Logger.Info(
			"waiting for previous devbox container to stop before commit",
			"devbox",
			devbox.Name,
			"containerID",
			previousContainerID,
		)
		if err := committer.WaitContainerStopped(ctx, previousContainerID, 30*time.Second); err != nil {
			h.Logger.Error(
				err,
				"failed waiting for previous devbox container to stop before commit",
				"devbox",
				devbox.Name,
				"containerID",
				previousContainerID,
			)
			return err
		}
		if err := committer.UnmountSnapshot(ctx, previousContainerID); err != nil {
			h.Logger.Error(
				err,
				"failed to detach previous devbox snapshot before commit",
				"devbox",
				devbox.Name,
				"containerID",
				previousContainerID,
			)
			return err
		}
	}

	imageExists, err := committer.ImageExists(ctx, commitImage)
	if err != nil {
		h.Logger.Error(err, "failed to check local commit image", "commitImage", commitImage)
		return err
	}
	if imageExists {
		h.Logger.Info(
			"commit image already exists locally, skip commit and push directly",
			"devbox",
			devbox.Name,
			"commitImage",
			commitImage,
		)
	} else {
		containerID, commitErr = committer.Commit(
			ctx,
			devbox.Name,
			devbox.Status.ContentID,
			baseImage,
			commitImage,
			devbox.Spec.StorageLimit,
		)
		if commitErr != nil {
			h.Logger.Error(commitErr, "failed to commit devbox", "devbox", devbox.Name)
			// Update commit status to failed on commit error with retry
			updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				latestDevbox := &devboxv1alpha2.Devbox{}
				if err := h.Client.Get(
					ctx,
					types.NamespacedName{Namespace: devbox.Namespace, Name: devbox.Name},
					latestDevbox,
				); err != nil {
					// If devbox is not found, return the error
					// RetryOnConflict will return this error immediately without retrying
					if apierrors.IsNotFound(err) {
						return err
					}
					return err
				}
				currentRecord, err := getCurrentCommitRecord(latestDevbox)
				if err != nil {
					return err
				}
				currentRecord.CommitStatus = devboxv1alpha2.CommitStatusFailed
				currentRecord.UpdateTime = metav1.Now()
				latestDevbox.SetCondition(metav1.Condition{
					Type:               devboxv1alpha2.DevboxConditionCommitInProgress,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: latestDevbox.Generation,
					Reason:             devboxv1alpha2.DevboxReasonCommitFailed,
					Message:            "commit workflow failed",
					LastTransitionTime: metav1.Now(),
				})
				return h.Client.Status().Update(ctx, latestDevbox)
			})
			if updateErr != nil {
				if apierrors.IsNotFound(updateErr) {
					h.Logger.Info(
						"devbox not found when updating commit status to failed",
						"devbox",
						devbox.Name,
					)
					return updateErr
				}
				h.Logger.Error(
					updateErr,
					"failed to update commit status to failed",
					"devbox",
					devbox.Name,
				)
			}
			return commitErr
		}
	}
	if err := h.Client.Get(
		ctx,
		types.NamespacedName{Namespace: devbox.Namespace, Name: devbox.Name},
		devbox,
	); err != nil {
		if apierrors.IsNotFound(err) {
			h.Logger.Info("devbox not found before push", "devbox", devbox.Name)
			return err
		}
		h.Logger.Error(err, "failed to get devbox", "devbox", devbox.Name)
		return err
	}
	if err := committer.Push(ctx, commitImage); err != nil {
		h.Logger.Error(err, "failed to push commit image", "commitImage", commitImage)
		// Update commit status to failed on push error with retry
		updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latestDevbox := &devboxv1alpha2.Devbox{}
			if err := h.Client.Get(
				ctx,
				types.NamespacedName{Namespace: devbox.Namespace, Name: devbox.Name},
				latestDevbox,
			); err != nil {
				// If devbox is not found, return the error
				if apierrors.IsNotFound(err) {
					return err
				}
				return err
			}
			currentRecord, err := getCurrentCommitRecord(latestDevbox)
			if err != nil {
				return err
			}
			currentRecord.CommitStatus = devboxv1alpha2.CommitStatusFailed
			currentRecord.UpdateTime = metav1.Now()
			latestDevbox.SetCondition(metav1.Condition{
				Type:               devboxv1alpha2.DevboxConditionCommitInProgress,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: latestDevbox.Generation,
				Reason:             devboxv1alpha2.DevboxReasonCommitFailed,
				Message:            "commit workflow failed (push error)",
				LastTransitionTime: metav1.Now(),
			})
			return h.Client.Status().Update(ctx, latestDevbox)
		})
		if updateErr != nil {
			if apierrors.IsNotFound(updateErr) {
				h.Logger.Info(
					"devbox not found when updating commit status to failed after push error",
					"devbox",
					devbox.Name,
				)
				return updateErr
			}
			h.Logger.Error(
				updateErr,
				"failed to update commit status to failed",
				"devbox",
				devbox.Name,
			)
		}
		return err
	}
	h.Logger.Info("push commit image success", "commitImage", commitImage)
	// step 2: update devbox commit record
	// step 3: update devbox status state to shutdown
	// step 4: add a new commit record for the new content id
	// make sure that always have a new commit record for shutdown state
	newContentID := uuid.New().String()
	newCommitImage := h.generateImageName(devbox)
	h.Logger.Info("update devbox status to shutdown", "devbox", devbox.Name)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestDevbox := &devboxv1alpha2.Devbox{}
		if err := h.Client.Get(
			ctx,
			types.NamespacedName{Namespace: devbox.Namespace, Name: devbox.Name},
			latestDevbox,
		); err != nil {
			// If devbox is not found, return the error to stop retrying
			if apierrors.IsNotFound(err) {
				return err
			}
			return err
		}
		currentRecord, err := getCurrentCommitRecord(latestDevbox)
		if err != nil {
			return err
		}
		if _, err := helper.EnsureCommitRecordRuntimeMetadata(
			ctx,
			h.Client,
			currentRecord,
			latestDevbox.Spec.RuntimeClassName,
		); err != nil {
			return err
		}
		currentRecord.CommitStatus = devboxv1alpha2.CommitStatusSuccess
		currentRecord.CommitTime = metav1.Now()
		latestDevbox.Status.State = targetState
		latestDevbox.Status.ContentID = newContentID
		if latestDevbox.Status.CommitRecords == nil {
			latestDevbox.Status.CommitRecords = make(devboxv1alpha2.CommitRecordMap)
		}
		latestDevbox.Status.CommitRecords[newContentID] = &devboxv1alpha2.CommitRecord{
			CommitStatus:     devboxv1alpha2.CommitStatusPending,
			Node:             "",
			BaseImage:        commitImage,
			CommitImage:      newCommitImage,
			GenerateTime:     metav1.Now(),
			RuntimeClassName: currentRecord.RuntimeClassName,
			RuntimeHandler:   currentRecord.RuntimeHandler,
			Snapshotter:      currentRecord.Snapshotter,
		}
		latestDevbox.Status.Node = ""
		// Commit succeeded; clear in-progress, and clear pending transition if synced.
		latestDevbox.SetCondition(metav1.Condition{
			Type:               devboxv1alpha2.DevboxConditionCommitInProgress,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: latestDevbox.Generation,
			Reason:             devboxv1alpha2.DevboxReasonCommitSucceeded,
			Message:            "commit workflow succeeded",
			LastTransitionTime: metav1.Now(),
		})
		if latestDevbox.Spec.State == latestDevbox.Status.State {
			latestDevbox.Status.ObservedGeneration = latestDevbox.Generation
			latestDevbox.SetCondition(metav1.Condition{
				Type:               devboxv1alpha2.DevboxConditionStateTransitionPending,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: latestDevbox.Generation,
				Reason:             devboxv1alpha2.DevboxReasonStateTransitionSynced,
				Message:            "spec.state matches status.state",
				LastTransitionTime: metav1.Now(),
			})
		}
		return h.Client.Status().Update(ctx, latestDevbox)
	}); err != nil {
		if apierrors.IsNotFound(err) {
			h.Logger.Info("devbox not found when updating status", "devbox", devbox.Name)
			return err
		}
		h.Logger.Error(err, "failed to update devbox status", "devbox", devbox.Name)
		return err
	}
	if baseImage != "" && baseImage != commitImage {
		// The freshly committed image becomes the next generation's base image,
		// so only the previous base image is eligible for local cleanup here.
		shouldRemoveBaseImage := true
		if previousContainerID != "" {
			exists, err := committer.ContainerExists(ctx, previousContainerID)
			if err != nil {
				h.Logger.Error(
					err,
					"failed to check previous container before base image cleanup",
					"devbox",
					devbox.Name,
					"containerID",
					previousContainerID,
					"baseImage",
					baseImage,
				)
				shouldRemoveBaseImage = false
			} else if exists {
				h.Logger.Info(
					"skip removing old base image because previous container still exists",
					"devbox",
					devbox.Name,
					"containerID",
					previousContainerID,
					"baseImage",
					baseImage,
				)
				shouldRemoveBaseImage = false
			}
		}
		if shouldRemoveBaseImage {
			removeImageNames = append(removeImageNames, baseImage)
		}
	}
	// step 5: set LV removable
	if containerID != "" {
		if err := committer.SetLvRemovable(ctx, containerID, oldContentID); err != nil {
			h.Logger.Error(
				err,
				"failed to set LV removable",
				"containerID",
				containerID,
				"contentID",
				oldContentID,
			)
		}
	} else {
		h.Logger.Info(
			"skip set LV removable because commit container was not created in this round",
			"devbox",
			devbox.Name,
			"contentID",
			oldContentID,
			"commitImage",
			commitImage,
		)
	}
	return nil
}

func (h *EventHandler) generateImageName(devbox *devboxv1alpha2.Devbox) string {
	now := time.Now()
	return fmt.Sprintf(
		"%s/%s/%s:%s-%s",
		h.CommitImageRegistry,
		devbox.Namespace,
		devbox.Name,
		rand.String(5),
		now.Format("2006-01-02-150405"),
	)
}

func (h *EventHandler) cleanupStorage(
	ctx context.Context,
	devboxName, contentID, baseImage, storageLimit, snapshotter, runtimeClass, runtimeHandler string,
) error {
	committer, err := h.getCommitterBySnapshotter(snapshotter)
	if err != nil {
		h.Logger.Error(
			err,
			"failed to select committer for cleanup",
			"devbox",
			devboxName,
			"contentID",
			contentID,
			"snapshotter",
			snapshotter,
			"runtimeClass",
			runtimeClass,
			"runtimeHandler",
			runtimeHandler,
		)
		return err
	}
	h.Logger.Info(
		"Starting Storage cleanup",
		"devbox",
		devboxName,
		"contentID",
		contentID,
		"baseImage",
		baseImage,
		"storageLimit",
		storageLimit,
		"snapshotter",
		snapshotter,
		"runtimeClass",
		runtimeClass,
		"runtimeHandler",
		runtimeHandler,
		"defaultBaseImage",
		h.DefaultBaseImage,
	)

	// create temp container
	containerID, err := committer.CreateContainer(
		ctx,
		fmt.Sprintf("temp-%s-%d", devboxName, time.Now().UnixMicro()),
		contentID,
		h.DefaultBaseImage,
		storageLimit,
	)
	if err != nil {
		h.Logger.Error(
			err,
			"failed to create temp container",
			"devbox",
			devboxName,
			"contentID",
			contentID,
			"defaultBaseImage",
			h.DefaultBaseImage,
		)
		return err
	}

	// make sure remove container
	defer func() {
		if cleanupErr := committer.RemoveContainers(
			ctx,
			[]string{containerID},
		); cleanupErr != nil {
			h.Logger.Error(
				cleanupErr,
				"failed to remove temporary container",
				"devbox",
				devboxName,
				"containerID",
				containerID,
			)
		} else {
			h.Logger.Info(
				"Successfully removed temporary container",
				"devbox",
				devboxName,
				"containerID",
				containerID,
			)
		}
	}()

	// remove storage
	if err := committer.SetLvRemovable(ctx, containerID, contentID); err != nil {
		h.Logger.Error(
			err,
			"failed to set Storage removable",
			"devbox",
			devboxName,
			"containerID",
			containerID,
			"contentID",
			contentID,
		)
		return fmt.Errorf("failed to set Storage removable: %w", err)
	}

	h.Logger.Info(
		"Successfully marked storage removable",
		"devbox",
		devboxName,
		"containerID",
		containerID,
		"contentID",
		contentID,
	)

	return nil
}

func normalizeContainerRuntimeID(containerID string) string {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return ""
	}
	parts := strings.SplitN(containerID, "://", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return containerID
}

func isImageInUseConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "image is being used by")
}

func getCurrentCommitRecord(devbox *devboxv1alpha2.Devbox) (*devboxv1alpha2.CommitRecord, error) {
	if devbox.Status.CommitRecords == nil {
		return nil, fmt.Errorf("commit records are empty for devbox %s", devbox.Name)
	}
	record := devbox.Status.CommitRecords[devbox.Status.ContentID]
	if record == nil {
		return nil, fmt.Errorf(
			"commit record not found for devbox %s contentID %s",
			devbox.Name,
			devbox.Status.ContentID,
		)
	}
	return record, nil
}
