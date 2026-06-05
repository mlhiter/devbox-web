package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	devboxv1alpha2 "github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHandleCreateDevboxAddsUpstreamLabel(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","upstreamID":"session-1","labels":[{"key":"app.kubernetes.io/component","value":"runtime"}]}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, resp.Code, resp.Body.String())
	}

	obj := &devboxv1alpha2.Devbox{}
	if err := srv.ctrlClient.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "ns-test", Name: "demo-devbox"}, obj); err != nil {
		t.Fatalf("get created devbox failed: %v", err)
	}
	if got := obj.Labels[devboxUpstreamIDLabelKey]; got != "session-1" {
		t.Fatalf("unexpected upstream label value: %q", got)
	}
	if got := obj.Labels["app.kubernetes.io/component"]; got != "runtime" {
		t.Fatalf("unexpected custom label value: %q", got)
	}
}

func TestHandleCreateDevboxRejectsInvalidUpstreamID(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","upstreamID":"bad/value"}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestHandleCreateDevboxWithLifecycleConfig(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","pauseAt":"2026-03-02T12:30:00Z","archiveAfterPauseTime":"2h"}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, resp.Code, resp.Body.String())
	}

	obj := &devboxv1alpha2.Devbox{}
	if err := srv.ctrlClient.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "ns-test", Name: "demo-devbox"}, obj); err != nil {
		t.Fatalf("get created devbox failed: %v", err)
	}
	if got := obj.Labels[devboxLifecycleLabelKey]; got != "true" {
		t.Fatalf("unexpected lifecycle label value: %q", got)
	}
	if got := obj.Annotations[devboxAnnotationPauseAt]; got != "2026-03-02T12:30:00Z" {
		t.Fatalf("unexpected pauseAt annotation: %q", got)
	}
	if got := obj.Annotations[devboxAnnotationArchiveAfterPauseTime]; got != "2h0m0s" {
		t.Fatalf("unexpected archiveAfterPauseTime annotation: %q", got)
	}
}

func TestHandleCreateDevboxWithEnv(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","env":{"FOO":"bar","DEVBOX_SDK_RUN_AS_ROOT":"false"}}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, resp.Code, resp.Body.String())
	}

	obj := &devboxv1alpha2.Devbox{}
	if err := srv.ctrlClient.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "ns-test", Name: "demo-devbox"}, obj); err != nil {
		t.Fatalf("get created devbox failed: %v", err)
	}
	envByName := make(map[string]string, len(obj.Spec.Config.Env))
	for _, item := range obj.Spec.Config.Env {
		envByName[item.Name] = item.Value
	}
	if got := envByName["FOO"]; got != "bar" {
		t.Fatalf("unexpected FOO env value: %q", got)
	}
	if got := envByName["DEVBOX_SDK_RUN_AS_ROOT"]; got != "false" {
		t.Fatalf("unexpected DEVBOX_SDK_RUN_AS_ROOT env value: %q", got)
	}
}

func TestHandleCreateDevboxWithImage(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","image":"registry.example.com/devbox/runtime:custom-v2"}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, resp.Code, resp.Body.String())
	}

	obj := &devboxv1alpha2.Devbox{}
	if err := srv.ctrlClient.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "ns-test", Name: "demo-devbox"}, obj); err != nil {
		t.Fatalf("get created devbox failed: %v", err)
	}
	if got := obj.Spec.Image; got != "registry.example.com/devbox/runtime:custom-v2" {
		t.Fatalf("unexpected image value: %q", got)
	}
}

func TestHandleCreateDevboxWithKubeAccess(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","kubeAccess":{"enabled":true,"roleTemplate":"admin"}}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, resp.Code, resp.Body.String())
	}

	obj := &devboxv1alpha2.Devbox{}
	if err := srv.ctrlClient.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "ns-test", Name: "demo-devbox"}, obj); err != nil {
		t.Fatalf("get created devbox failed: %v", err)
	}
	if obj.Spec.KubeAccess == nil {
		t.Fatalf("expected kubeAccess to be set")
	}
	if !obj.Spec.KubeAccess.Enabled {
		t.Fatalf("expected kubeAccess.enabled to be true")
	}
	if got := obj.Spec.KubeAccess.RoleTemplate; got != devboxv1alpha2.KubeAccessRoleTemplateAdmin {
		t.Fatalf("unexpected kubeAccess.roleTemplate: %q", got)
	}
}

func TestHandleCreateDevboxRejectsBlankImage(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","image":"   "}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestHandleCreateDevboxRejectsInvalidKubeAccessRoleTemplate(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","kubeAccess":{"enabled":true,"roleTemplate":"owner"}}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestHandleCreateDevboxRejectsInvalidEnvName(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","env":{"1BAD":"x"}}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestHandleCreateDevboxRejectsInvalidArchiveAfterPauseTime(t *testing.T) {
	srv := newTestAPIServer(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox",
		bytes.NewBufferString(`{"name":"demo-devbox","archiveAfterPauseTime":"bad"}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestHandleListDevboxesFilterByUpstreamID(t *testing.T) {
	t1 := metav1.NewTime(time.Unix(1_700_000_000, 0).UTC())
	t2 := metav1.NewTime(time.Unix(1_700_000_100, 0).UTC())

	srv := newTestAPIServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "db-a",
				Namespace:         "ns-test",
				CreationTimestamp: t1,
				Labels: map[string]string{
					devboxUpstreamIDLabelKey: "session-a",
				},
			},
			Spec: devboxv1alpha2.DevboxSpec{
				State: devboxv1alpha2.DevboxStateRunning,
			},
			Status: devboxv1alpha2.DevboxStatus{
				State: devboxv1alpha2.DevboxStateRunning,
				Phase: devboxv1alpha2.DevboxPhaseRunning,
			},
		},
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "db-b",
				Namespace:         "ns-test",
				CreationTimestamp: t2,
				Labels: map[string]string{
					devboxUpstreamIDLabelKey: "session-b",
				},
			},
			Spec: devboxv1alpha2.DevboxSpec{
				State: devboxv1alpha2.DevboxStatePaused,
			},
			Status: devboxv1alpha2.DevboxStatus{
				State: devboxv1alpha2.DevboxStatePaused,
				Phase: devboxv1alpha2.DevboxPhasePaused,
			},
		},
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-c",
				Namespace: "ns-other",
				Labels: map[string]string{
					devboxUpstreamIDLabelKey: "session-a",
				},
			},
			Spec: devboxv1alpha2.DevboxSpec{
				State: devboxv1alpha2.DevboxStateRunning,
			},
			Status: devboxv1alpha2.DevboxStatus{
				State: devboxv1alpha2.DevboxStateRunning,
				Phase: devboxv1alpha2.DevboxPhaseRunning,
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devbox?upstreamID=session-a", nil)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Items []listDevboxItem `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(payload.Data.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(payload.Data.Items))
	}
	item := payload.Data.Items[0]
	if item.Name != "db-a" {
		t.Fatalf("unexpected item name: %s", item.Name)
	}
	if item.State.Spec != string(devboxv1alpha2.DevboxStateRunning) {
		t.Fatalf("unexpected item state.spec: %s", item.State.Spec)
	}
	if item.CreationTimestamp != t1.UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected creationTimestamp: %s", item.CreationTimestamp)
	}
}

func TestHandleListDevboxesRejectsInvalidUpstreamID(t *testing.T) {
	srv := newTestAPIServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devbox?upstreamID=bad/value", nil)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestHandleRefreshDevboxPauseAt(t *testing.T) {
	srv := newTestAPIServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-a",
				Namespace: "ns-test",
				Labels: map[string]string{
					devboxLifecycleLabelKey: "true",
				},
				Annotations: map[string]string{
					devboxAnnotationPauseAt:               "2026-03-02T08:00:00Z",
					devboxAnnotationArchiveAfterPauseTime: "1h0m0s",
					devboxAnnotationPausedAt:              "2026-03-02T08:01:00Z",
				},
			},
		},
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox/db-a/pause/refresh",
		bytes.NewBufferString(`{"pauseAt":"2026-03-03T09:00:00Z"}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	latest := &devboxv1alpha2.Devbox{}
	if err := srv.ctrlClient.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "ns-test", Name: "db-a"}, latest); err != nil {
		t.Fatalf("get refreshed devbox failed: %v", err)
	}
	if got := latest.Annotations[devboxAnnotationPauseAt]; got != "2026-03-03T09:00:00Z" {
		t.Fatalf("unexpected pauseAt annotation: %q", got)
	}
	if _, exists := latest.Annotations[devboxAnnotationPausedAt]; exists {
		t.Fatalf("pausedAt annotation should be cleared on refresh")
	}
	if got := latest.Annotations[devboxAnnotationPauseRefreshAt]; got == "" {
		t.Fatalf("pauseRefreshAt annotation should be set")
	}
}

func TestHandleGetDevboxInfoIncludesGateway(t *testing.T) {
	srv := newTestAPIServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Spec: devboxv1alpha2.DevboxSpec{
				State: devboxv1alpha2.DevboxStateRunning,
			},
			Status: devboxv1alpha2.DevboxStatus{
				State: devboxv1alpha2.DevboxStateRunning,
				Phase: devboxv1alpha2.DevboxPhaseRunning,
				Network: devboxv1alpha2.NetworkStatus{
					UniqueID: "demo-unique-id",
				},
				Conditions: []metav1.Condition{
					{
						Type:    devboxv1alpha2.DevboxConditionPodReady,
						Status:  metav1.ConditionFalse,
						Reason:  devboxv1alpha2.DevboxReasonStorageFull,
						Message: "no space left on device",
					},
				},
				LastContainerStatus: corev1.ContainerStatus{
					Name: "demo-devbox",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CreateContainerError",
							Message: "no space left on device",
						},
					},
				},
			},
		},
	)
	srv.cfg.Gateway = GatewayConfig{
		Domain:     "devbox-gateway.staging-usw-1.sealos.io",
		PathPrefix: "/codex",
		Port:       1317,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-devbox",
			Namespace: "ns-test",
		},
		Data: map[string][]byte{
			"SEALOS_DEVBOX_PRIVATE_KEY": []byte("fake-private-key"),
			devboxJWTSecretKey:          []byte("devbox-jwt-secret"),
		},
	}
	if err := srv.ctrlClient.Create(context.Background(), secret); err != nil {
		t.Fatalf("create secret failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devbox/demo-devbox", nil)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Name    string `json:"name"`
			Gateway struct {
				URL      string `json:"url"`
				Token    string `json:"token"`
				Port     int    `json:"port"`
				UniqueID string `json:"uniqueID"`
			} `json:"gateway"`
			CodeServerGateway struct {
				URL      string `json:"url"`
				Password string `json:"password"`
				Port     int    `json:"port"`
				UniqueID string `json:"uniqueID"`
			} `json:"codeServerGateway"`
			Conditions          []metav1.Condition     `json:"conditions"`
			LastContainerStatus corev1.ContainerStatus `json:"lastContainerStatus"`
			SSH                 struct {
				PrivateKeyBase64 string `json:"privateKeyBase64"`
			} `json:"ssh"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.Data.Name != "demo-devbox" {
		t.Fatalf("unexpected name: %s", payload.Data.Name)
	}
	if payload.Data.Gateway.Port != 1317 {
		t.Fatalf("unexpected gateway port: %d", payload.Data.Gateway.Port)
	}
	if payload.Data.Gateway.URL != "https://devbox-gateway.staging-usw-1.sealos.io/codex/demo-unique-id" {
		t.Fatalf("unexpected gateway url: %s", payload.Data.Gateway.URL)
	}
	if payload.Data.CodeServerGateway.Port != 1318 {
		t.Fatalf("unexpected code-server gateway port: %d", payload.Data.CodeServerGateway.Port)
	}
	if payload.Data.CodeServerGateway.URL != "https://devbox-gateway.staging-usw-1.sealos.io/code-server/demo-unique-id" {
		t.Fatalf("unexpected code-server gateway url: %s", payload.Data.CodeServerGateway.URL)
	}
	claims := decodeGatewayTokenClaimsForTest(t, payload.Data.Gateway.Token, "devbox-jwt-secret", time.Now().UTC())
	if claims.Namespace != "ns-test" {
		t.Fatalf("unexpected gateway token namespace: %s", claims.Namespace)
	}
	if claims.DevboxName != "demo-devbox" {
		t.Fatalf("unexpected gateway token devboxName: %s", claims.DevboxName)
	}
	if claims.Exp <= claims.Iat {
		t.Fatalf("unexpected gateway token lifetime: iat=%d exp=%d", claims.Iat, claims.Exp)
	}
	if payload.Data.Gateway.UniqueID != "demo-unique-id" {
		t.Fatalf("unexpected uniqueID: %s", payload.Data.Gateway.UniqueID)
	}
	if payload.Data.CodeServerGateway.Password != "devbox-jwt-secret" {
		t.Fatalf("unexpected code-server gateway password: %s", payload.Data.CodeServerGateway.Password)
	}
	if payload.Data.CodeServerGateway.UniqueID != "demo-unique-id" {
		t.Fatalf("unexpected code-server gateway uniqueID: %s", payload.Data.CodeServerGateway.UniqueID)
	}
	if len(payload.Data.Conditions) != 1 {
		t.Fatalf("expected one condition, got %d", len(payload.Data.Conditions))
	}
	if payload.Data.Conditions[0].Type != devboxv1alpha2.DevboxConditionPodReady ||
		payload.Data.Conditions[0].Reason != devboxv1alpha2.DevboxReasonStorageFull {
		t.Fatalf("unexpected conditions: %+v", payload.Data.Conditions)
	}
	if payload.Data.LastContainerStatus.State.Waiting == nil ||
		payload.Data.LastContainerStatus.State.Waiting.Message != "no space left on device" {
		t.Fatalf("unexpected lastContainerStatus: %+v", payload.Data.LastContainerStatus)
	}
	entry, ok := srv.getGatewayIndex("demo-unique-id")
	if !ok {
		t.Fatalf("expected gateway index entry for uniqueID")
	}
	if entry.Name != "demo-devbox" || entry.Namespace != "ns-test" {
		t.Fatalf("unexpected gateway index entry: %+v", entry)
	}
	if payload.Data.SSH.PrivateKeyBase64 != base64.StdEncoding.EncodeToString([]byte("fake-private-key")) {
		t.Fatalf("unexpected private key base64: %s", payload.Data.SSH.PrivateKeyBase64)
	}
}

func TestHandleGetDevboxInfoWaitsForSecretProvisioning(t *testing.T) {
	srv := newTestAPIServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Spec: devboxv1alpha2.DevboxSpec{
				State: devboxv1alpha2.DevboxStateRunning,
			},
			Status: devboxv1alpha2.DevboxStatus{
				State: devboxv1alpha2.DevboxStateRunning,
				Phase: devboxv1alpha2.DevboxPhaseRunning,
				Network: devboxv1alpha2.NetworkStatus{
					UniqueID: "demo-unique-id",
				},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Data: map[string][]byte{
				"SEALOS_DEVBOX_PRIVATE_KEY": []byte("fake-private-key"),
				devboxJWTSecretKey:          []byte("devbox-jwt-secret"),
			},
		},
	)
	srv.cfg.Gateway = GatewayConfig{
		Domain:     "devbox-gateway.staging-usw-1.sealos.io",
		PathPrefix: "/codex",
		Port:       1317,
	}
	srv.devboxSecretWaitTimeout = 50 * time.Millisecond
	srv.devboxSecretRetryInterval = time.Millisecond
	srv.ctrlClient = &transientSecretNotFoundClient{
		Client:    srv.ctrlClient,
		namespace: "ns-test",
		name:      "demo-devbox",
		remaining: 2,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devbox/demo-devbox", nil)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	var payload struct {
		Data struct {
			Gateway *struct {
				URL string `json:"url"`
			} `json:"gateway"`
			SSH struct {
				PrivateKeyBase64 string `json:"privateKeyBase64"`
			} `json:"ssh"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.Data.Gateway == nil {
		t.Fatalf("expected gateway info after secret becomes available")
	}
	if payload.Data.SSH.PrivateKeyBase64 != base64.StdEncoding.EncodeToString([]byte("fake-private-key")) {
		t.Fatalf("unexpected private key base64: %s", payload.Data.SSH.PrivateKeyBase64)
	}
}

func TestHandleGetDevboxInfoReturnsPartialWhileSecretIsProvisioning(t *testing.T) {
	srv := newTestAPIServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Spec: devboxv1alpha2.DevboxSpec{
				State: devboxv1alpha2.DevboxStateRunning,
			},
			Status: devboxv1alpha2.DevboxStatus{
				State: devboxv1alpha2.DevboxStateRunning,
				Phase: devboxv1alpha2.DevboxPhasePending,
				Network: devboxv1alpha2.NetworkStatus{
					UniqueID: "demo-unique-id",
				},
			},
		},
	)
	srv.cfg.Gateway = GatewayConfig{
		Domain:     "devbox-gateway.staging-usw-1.sealos.io",
		PathPrefix: "/codex",
		Port:       1317,
	}
	srv.devboxSecretWaitTimeout = 5 * time.Millisecond
	srv.devboxSecretRetryInterval = time.Millisecond

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devbox/demo-devbox", nil)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	var payload struct {
		Data struct {
			Gateway *struct {
				URL string `json:"url"`
			} `json:"gateway"`
			State struct {
				Phase string `json:"phase"`
			} `json:"state"`
			SSH struct {
				PrivateKeyBase64 string `json:"privateKeyBase64"`
			} `json:"ssh"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.Data.Gateway != nil {
		t.Fatalf("expected gateway info to be omitted while credentials are pending")
	}
	if payload.Data.SSH.PrivateKeyBase64 != "" {
		t.Fatalf("expected empty private key while credentials are pending, got %q", payload.Data.SSH.PrivateKeyBase64)
	}
	if payload.Data.State.Phase != string(devboxv1alpha2.DevboxPhasePending) {
		t.Fatalf("unexpected phase: %s", payload.Data.State.Phase)
	}
}

func newTestAPIServer(t *testing.T, objs ...ctrlclient.Object) *apiServer {
	t.Helper()

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("add core scheme failed: %v", err)
	}
	if err := devboxv1alpha2.AddToScheme(s); err != nil {
		t.Fatalf("add devbox scheme failed: %v", err)
	}

	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		Build()

	return &apiServer{
		cfg: ServerConfig{
			JWTSigningKey: "test-secret",
			SSH: SSHConnectionConfig{
				User:                "devbox",
				Host:                "staging-usw-1.sealos.io",
				Port:                2233,
				PrivateKeySecretKey: "SEALOS_DEVBOX_PRIVATE_KEY",
			},
			CreateResource: CreateDevboxResourceConfig{
				CPU:          "1000m",
				Memory:       "1Gi",
				StorageLimit: "10Gi",
				Image:        "registry.example.com/devbox/runtime:latest",
			},
		},
		ctrlClient:                c,
		logger:                    newLogger(io.Discard, slog.LevelDebug),
		devboxSecretWaitTimeout:   20 * time.Millisecond,
		devboxSecretRetryInterval: time.Millisecond,
	}
}

type transientSecretNotFoundClient struct {
	ctrlclient.Client
	mu        sync.Mutex
	namespace string
	name      string
	remaining int
}

func (c *transientSecretNotFoundClient) Get(
	ctx context.Context,
	key ctrlclient.ObjectKey,
	obj ctrlclient.Object,
	opts ...ctrlclient.GetOption,
) error {
	if _, ok := obj.(*corev1.Secret); ok && key.Namespace == c.namespace && key.Name == c.name {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.remaining > 0 {
			c.remaining--
			return apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, key.Name)
		}
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func issueBearerTokenForNamespace(t *testing.T, namespace string) string {
	t.Helper()
	now := time.Now().UTC()
	token := issueTestJWT(t, "test-secret", jwtClaims{
		Namespace: namespace,
		Iat:       now.Unix() - 10,
		Nbf:       now.Unix() - 5,
		Exp:       now.Unix() + 3600,
	})
	return "Bearer " + token
}

func decodeGatewayTokenClaimsForTest(t *testing.T, token string, signingKey string, now time.Time) gatewayTokenClaims {
	t.Helper()

	claims, err := verifyJWTToken(token, signingKey, now)
	if err != nil {
		t.Fatalf("verify gateway token failed: %v", err)
	}
	if claims.Namespace == "" {
		t.Fatalf("expected namespace claim in gateway token")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected jwt token format")
	}
	claimBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode gateway token claims failed: %v", err)
	}

	var gatewayClaims gatewayTokenClaims
	if err := json.Unmarshal(claimBytes, &gatewayClaims); err != nil {
		t.Fatalf("unmarshal gateway token claims failed: %v", err)
	}
	return gatewayClaims
}
