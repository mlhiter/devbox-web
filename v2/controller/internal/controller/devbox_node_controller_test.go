package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	devboxv1alpha2 "github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	"github.com/sealos-apps/devbox/v2/controller/internal/commit"
	"github.com/sealos-apps/devbox/v2/controller/internal/controller/helper"
	"github.com/sealos-apps/devbox/v2/controller/label"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeNodeCommitter struct {
	createContainerCalls int
	commitCalls          int
	pushCalls            int
	setRemovableCalls    int
	removeContainerCalls int
}

func (f *fakeNodeCommitter) CreateContainer(
	context.Context,
	string,
	string,
	string,
	string,
) (string, error) {
	f.createContainerCalls++
	return "fake-container-id", nil
}

func (f *fakeNodeCommitter) Commit(context.Context, string, string, string, string, string) (string, error) {
	f.commitCalls++
	return "fake-container-id", nil
}

func (f *fakeNodeCommitter) ContainerExists(context.Context, string) (bool, error) {
	return false, nil
}

func (f *fakeNodeCommitter) ImageExists(context.Context, string) (bool, error) {
	return false, nil
}

func (f *fakeNodeCommitter) Push(context.Context, string) error {
	f.pushCalls++
	return nil
}

func (f *fakeNodeCommitter) RemoveImages(context.Context, []string, bool, bool) error {
	return nil
}

func (f *fakeNodeCommitter) RemoveContainers(context.Context, []string) error {
	f.removeContainerCalls++
	return nil
}

func (f *fakeNodeCommitter) InitializeGC(context.Context) error {
	return nil
}

func (f *fakeNodeCommitter) SetLvRemovable(context.Context, string, string) error {
	f.setRemovableCalls++
	return nil
}

func (f *fakeNodeCommitter) UnmountSnapshot(context.Context, string) error {
	return nil
}

func (f *fakeNodeCommitter) WaitContainerStopped(context.Context, string, time.Duration) error {
	return nil
}

var _ = Describe("Devbox Node Worker", func() {
	const (
		namespace = "default"
		nodeName  = "test-node"
	)

	ctx := context.Background()

	newNodeWorkerDevbox := func(name string, state, targetState devboxv1alpha2.DevboxState, contentID string) *devboxv1alpha2.Devbox {
		return &devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: devboxv1alpha2.DevboxSpec{
				State: targetState,
				Resource: corev1.ResourceList{
					corev1.ResourceCPU:    apiresource.MustParse("1"),
					corev1.ResourceMemory: apiresource.MustParse("1Gi"),
				},
				Image:        "busybox:latest",
				Config:       devboxv1alpha2.Config{},
				StorageLimit: "10Gi",
				NetworkSpec:  devboxv1alpha2.NetworkSpec{Type: devboxv1alpha2.NetworkTypeTailnet},
			},
			Status: devboxv1alpha2.DevboxStatus{
				ContentID: contentID,
				State:     state,
				CommitRecords: devboxv1alpha2.CommitRecordMap{
					contentID: {
						Node:             nodeName,
						BaseImage:        "busybox:latest",
						CommitImage:      "registry.example/devbox:commit",
						CommitStatus:     devboxv1alpha2.CommitStatusPending,
						RuntimeClassName: devboxv1alpha2.RuntimeClassDevboxRunc,
					},
				},
			},
		}
	}

	persistNodeWorkerStatus := func(key client.ObjectKey, status devboxv1alpha2.DevboxStatus) {
		latestDevbox := &devboxv1alpha2.Devbox{}
		Expect(k8sClient.Get(ctx, key, latestDevbox)).To(Succeed())
		latestDevbox.Status = status
		Expect(k8sClient.Status().Update(ctx, latestDevbox)).To(Succeed())
	}

	createDevboxPod := func(name string) *corev1.Pod {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       name,
					"app.kubernetes.io/managed-by": label.DefaultManagedBy,
					"app.kubernetes.io/part-of":    devboxv1alpha2.LabelDevBoxPartOf,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "devbox", Image: "busybox:latest"}},
			},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		return pod
	}

	newNodeReconciler := func(fakeCommitter *fakeNodeCommitter) *DevboxNodeReconciler {
		return &DevboxNodeReconciler{
			Client:   k8sClient,
			NodeName: nodeName,
			Handler: &EventHandler{
				Client:              k8sClient,
				Logger:              ctrl.Log.WithName("test-node-worker"),
				CommitImageRegistry: "registry.example",
				DefaultBaseImage:    "alpine:3.19",
				Committers: map[string]commit.Committer{
					commit.DefaultDevboxSnapshotter: fakeCommitter,
				},
			},
		}
	}

	It("waits for pod deletion before committing node-local content", func() {
		resourceName := fmt.Sprintf("test-node-commit-%s", rand.String(5))
		key := client.ObjectKey{Name: resourceName, Namespace: namespace}
		contentID := "content-" + rand.String(5)
		devbox := newNodeWorkerDevbox(
			resourceName,
			devboxv1alpha2.DevboxStateRunning,
			devboxv1alpha2.DevboxStateStopped,
			contentID,
		)
		initialStatus := devbox.Status
		Expect(k8sClient.Create(ctx, devbox)).To(Succeed())
		persistNodeWorkerStatus(key, initialStatus)
		createDevboxPod(resourceName)

		fakeCommitter := &fakeNodeCommitter{}
		reconciler := newNodeReconciler(fakeCommitter)

		result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(10 * time.Second))
		Expect(fakeCommitter.commitCalls).To(Equal(0))
		Expect(fakeCommitter.pushCalls).To(Equal(0))

		pod := &corev1.Pod{}
		Expect(k8sClient.Get(ctx, key, pod)).To(Succeed())
		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, key, &corev1.Pod{})
			return apierrors.IsNotFound(err)
		}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())

		result, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))
		Expect(fakeCommitter.commitCalls).To(Equal(1))
		Expect(fakeCommitter.pushCalls).To(Equal(1))

		latestDevbox := &devboxv1alpha2.Devbox{}
		Expect(k8sClient.Get(ctx, key, latestDevbox)).To(Succeed())
		Expect(latestDevbox.Status.State).To(Equal(devboxv1alpha2.DevboxStateStopped))
		Expect(latestDevbox.Status.ContentID).NotTo(Equal(contentID))
	})

	It("waits for pod deletion before cleaning node-local storage", func() {
		resourceName := fmt.Sprintf("test-node-cleanup-%s", rand.String(5))
		key := client.ObjectKey{Name: resourceName, Namespace: namespace}
		contentID := "content-" + rand.String(5)
		devbox := &devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:       resourceName,
				Namespace:  namespace,
				Finalizers: []string{devboxv1alpha2.FinalizerName},
			},
			Spec: devboxv1alpha2.DevboxSpec{
				State: devboxv1alpha2.DevboxStateRunning,
				Resource: corev1.ResourceList{
					corev1.ResourceCPU:    apiresource.MustParse("1"),
					corev1.ResourceMemory: apiresource.MustParse("1Gi"),
				},
				Image:        "busybox:latest",
				Config:       devboxv1alpha2.Config{},
				StorageLimit: "10Gi",
				NetworkSpec:  devboxv1alpha2.NetworkSpec{Type: devboxv1alpha2.NetworkTypeTailnet},
			},
		}
		Expect(k8sClient.Create(ctx, devbox)).To(Succeed())

		latestDevbox := &devboxv1alpha2.Devbox{}
		Expect(k8sClient.Get(ctx, key, latestDevbox)).To(Succeed())
		latestDevbox.Status.ContentID = contentID
		latestDevbox.Status.State = devboxv1alpha2.DevboxStateRunning
		latestDevbox.Status.CommitRecords = devboxv1alpha2.CommitRecordMap{
			contentID: {
				Node:             nodeName,
				BaseImage:        "busybox:latest",
				CommitImage:      "registry.example/devbox:commit",
				CommitStatus:     devboxv1alpha2.CommitStatusPending,
				RuntimeClassName: devboxv1alpha2.RuntimeClassDevboxRunc,
			},
		}
		Expect(k8sClient.Status().Update(ctx, latestDevbox)).To(Succeed())
		Expect(k8sClient.Get(ctx, key, latestDevbox)).To(Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
				Labels:    helper.GeneratePodLabels(latestDevbox),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "devbox", Image: "busybox:latest"}},
			},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		fakeCommitter := &fakeNodeCommitter{}
		reconciler := newNodeReconciler(fakeCommitter)
		record := latestDevbox.Status.CommitRecords[contentID]

		err := reconciler.cleanupDeletedDevbox(ctx, latestDevbox, record)
		Expect(errors.Is(err, errWaitingForDevboxPodDeletion)).To(BeTrue())
		Expect(fakeCommitter.createContainerCalls).To(Equal(0))

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), &corev1.Pod{})
			return apierrors.IsNotFound(err)
		}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())

		Expect(reconciler.cleanupDeletedDevbox(ctx, latestDevbox, record)).To(Succeed())
		Expect(fakeCommitter.createContainerCalls).To(Equal(1))
		Expect(fakeCommitter.setRemovableCalls).To(Equal(1))
		Expect(fakeCommitter.removeContainerCalls).To(Equal(1))

		Expect(k8sClient.Get(ctx, key, latestDevbox)).To(Succeed())
		Expect(latestDevbox.Finalizers).NotTo(ContainElement(devboxv1alpha2.FinalizerName))
	})
})
