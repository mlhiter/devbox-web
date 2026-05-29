package controller

import (
	"testing"
	"time"

	devboxv1alpha2 "github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsDevBoxReleaseTimedOut(t *testing.T) {
	now := time.Now()
	release := &devboxv1alpha2.DevBoxRelease{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.NewTime(now.Add(-devBoxReleaseTimeout - time.Second)),
		},
		Status: devboxv1alpha2.DevBoxReleaseStatus{
			Phase: devboxv1alpha2.DevBoxReleasePhasePending,
		},
	}

	if !isDevBoxReleaseTimedOut(release, now) {
		t.Fatal("expected pending release older than timeout to time out")
	}

	release.Status.Phase = devboxv1alpha2.DevBoxReleasePhaseSuccess
	if isDevBoxReleaseTimedOut(release, now) {
		t.Fatal("expected successful release not to time out")
	}

	release.Status.Phase = devboxv1alpha2.DevBoxReleasePhasePending
	release.CreationTimestamp = metav1.NewTime(now.Add(-devBoxReleaseTimeout + time.Second))
	if isDevBoxReleaseTimedOut(release, now) {
		t.Fatal("expected recent pending release not to time out")
	}
}
