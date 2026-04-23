package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	devboxv1alpha2 "github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMapSDKServerBusinessStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected int
	}{
		{status: 1400, expected: http.StatusBadRequest},
		{status: 1422, expected: http.StatusBadRequest},
		{status: 1401, expected: http.StatusInternalServerError},
		{status: 1403, expected: http.StatusInternalServerError},
		{status: 1404, expected: http.StatusNotFound},
		{status: 1409, expected: http.StatusConflict},
		{status: 1500, expected: http.StatusInternalServerError},
		{status: sdkServerStatusOperation, expected: http.StatusBadRequest},
		{status: 9999, expected: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		got := mapSDKServerBusinessStatus(tt.status)
		if got != tt.expected {
			t.Fatalf("status=%d expected=%d got=%d", tt.status, tt.expected, got)
		}
	}
}

func TestMapSDKServerHTTPStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected int
	}{
		{status: http.StatusBadRequest, expected: http.StatusBadRequest},
		{status: http.StatusNotFound, expected: http.StatusNotFound},
		{status: http.StatusConflict, expected: http.StatusConflict},
		{status: http.StatusGatewayTimeout, expected: http.StatusGatewayTimeout},
		{status: http.StatusUnauthorized, expected: http.StatusInternalServerError},
		{status: http.StatusForbidden, expected: http.StatusInternalServerError},
		{status: http.StatusBadGateway, expected: http.StatusInternalServerError},
		{status: http.StatusServiceUnavailable, expected: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		got := mapSDKServerHTTPStatus(tt.status)
		if got != tt.expected {
			t.Fatalf("status=%d expected=%d got=%d", tt.status, tt.expected, got)
		}
	}
}

func TestIsSDKServerTimeout(t *testing.T) {
	if !isSDKServerTimeout(1500, "Process execution timed out") {
		t.Fatalf("expected timeout message to be recognized")
	}
	if isSDKServerTimeout(1500, "other internal error") {
		t.Fatalf("expected non-timeout message to be rejected")
	}
	if isSDKServerTimeout(1400, "timed out") {
		t.Fatalf("expected non-1500 status to be rejected")
	}
}

func TestResolveRunningPodAndSDKServer(t *testing.T) {
	srv := newTestAPIServerForSDKServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
				UID:       "devbox-uid-v1",
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "demo-container"},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.0.0.8",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Data: map[string][]byte{
				devboxJWTSecretKey: []byte("jwt-secret"),
			},
		},
	)

	pod, container, baseURL, token, err := srv.resolveRunningPodAndSDKServer(
		context.Background(),
		"ns-test",
		"demo-devbox",
		"",
	)
	if err != nil {
		t.Fatalf("resolveRunningPodAndSDKServer failed: %v", err)
	}
	if pod.Name != "demo-devbox" {
		t.Fatalf("unexpected pod: %s", pod.Name)
	}
	if container != "demo-container" {
		t.Fatalf("unexpected container: %s", container)
	}
	if baseURL != "http://10.0.0.8:9757" {
		t.Fatalf("unexpected baseURL: %s", baseURL)
	}
	if token != "jwt-secret" {
		t.Fatalf("unexpected token: %s", token)
	}
}

func TestResolveDevboxJWTSecretCacheKeyUsesUID(t *testing.T) {
	srv := newTestAPIServerForSDKServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
				UID:       "devbox-uid-v2",
			},
		},
	)

	cacheKey, err := srv.resolveDevboxJWTSecretCacheKey(context.Background(), "ns-test", "demo-devbox")
	if err != nil {
		t.Fatalf("resolve cache key failed: %v", err)
	}
	if cacheKey != "ns-test/devbox-uid-v2" {
		t.Fatalf("unexpected cache key: %s", cacheKey)
	}
}

func TestResolveRunningPodAndSDKServerMissingPodIP(t *testing.T) {
	srv := newTestAPIServerForSDKServer(
		t,
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "demo-container"},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Data: map[string][]byte{
				devboxJWTSecretKey: []byte("jwt-secret"),
			},
		},
	)

	_, _, _, _, err := srv.resolveRunningPodAndSDKServer(
		context.Background(),
		"ns-test",
		"demo-devbox",
		"",
	)
	if err == nil {
		t.Fatalf("expected missing podIP error")
	}
	if !strings.Contains(err.Error(), "podIP") {
		t.Fatalf("expected error to mention podIP, got: %v", err)
	}
}

func TestBuildSDKServerExecSyncRequestWithoutStdin(t *testing.T) {
	req := execDevboxRequest{
		Command:        []string{"ls", "-la"},
		TimeoutSeconds: 42,
	}

	got := buildSDKServerExecSyncRequest(req)
	if got.Command != "ls" {
		t.Fatalf("unexpected command: %s", got.Command)
	}
	if len(got.Args) != 1 || got.Args[0] != "-la" {
		t.Fatalf("unexpected args: %#v", got.Args)
	}
	if got.Timeout != 42 {
		t.Fatalf("unexpected timeout: %d", got.Timeout)
	}
	if got.Env != nil {
		t.Fatalf("env should be nil when stdin is empty")
	}
}

func TestBuildSDKServerExecSyncRequestWithStdin(t *testing.T) {
	req := execDevboxRequest{
		Command:        []string{"cat"},
		Stdin:          "line-1\nline-2\n",
		TimeoutSeconds: 30,
	}

	got := buildSDKServerExecSyncRequest(req)
	if got.Command != "/bin/sh" {
		t.Fatalf("unexpected command: %s", got.Command)
	}
	if len(got.Args) != 4 {
		t.Fatalf("unexpected args length: %#v", got.Args)
	}
	if got.Args[0] != "-c" || got.Args[2] != "--" || got.Args[3] != "cat" {
		t.Fatalf("unexpected args: %#v", got.Args)
	}
	if !strings.Contains(got.Args[1], "SEALOS_DEVBOX_STDIN") {
		t.Fatalf("stdin wrapper script missing env key: %s", got.Args[1])
	}
	if got.Timeout != 30 {
		t.Fatalf("unexpected timeout: %d", got.Timeout)
	}
	if got.Env[sdkServerStdinEnvKey] != req.Stdin {
		t.Fatalf("unexpected stdin env value: %q", got.Env[sdkServerStdinEnvKey])
	}
}

func TestHandleExecDevboxWithStdinCompatibility(t *testing.T) {
	srv := newTestAPIServerForSDKServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
				UID:       "devbox-uid-v1",
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "demo-container"},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.0.0.8",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Data: map[string][]byte{
				devboxJWTSecretKey: []byte("sdk-token"),
			},
		},
	)
	srv.cfg.JWTSigningKey = "test-secret"

	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "http://10.0.0.8:9757/api/v1/process/exec-sync" {
			t.Fatalf("unexpected upstream URL: %s", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer sdk-token" {
			t.Fatalf("unexpected upstream token: %s", got)
		}

		var upstreamReq sdkServerExecSyncRequest
		if err := json.NewDecoder(req.Body).Decode(&upstreamReq); err != nil {
			t.Fatalf("decode upstream req failed: %v", err)
		}
		if upstreamReq.Command != "/bin/sh" {
			t.Fatalf("stdin wrapper should use /bin/sh, got: %s", upstreamReq.Command)
		}
		if len(upstreamReq.Args) != 4 || upstreamReq.Args[0] != "-c" || upstreamReq.Args[2] != "--" || upstreamReq.Args[3] != "cat" {
			t.Fatalf("unexpected wrapped args: %#v", upstreamReq.Args)
		}
		if upstreamReq.Env[sdkServerStdinEnvKey] != "hello\n" {
			t.Fatalf("stdin env mismatch: %q", upstreamReq.Env[sdkServerStdinEnvKey])
		}

		body := `{"status":0,"message":"success","stdout":"hello\n","stderr":"","exitCode":0}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	defer func() {
		http.DefaultClient.Transport = originalTransport
	}()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox/demo-devbox/exec",
		bytes.NewBufferString(`{"command":["cat"],"stdin":"hello\n","timeoutSeconds":30}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			ExitCode int    `json:"exitCode"`
			Stdout   string `json:"stdout"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.Data.ExitCode != 0 {
		t.Fatalf("unexpected exitCode: %d", payload.Data.ExitCode)
	}
	if payload.Data.Stdout != "hello\n" {
		t.Fatalf("unexpected stdout: %q", payload.Data.Stdout)
	}
}

func TestWaitForSDKServerReachableRetriesConnectionRefused(t *testing.T) {
	srv := newTestAPIServerForSDKServer(t)

	attempts := 0
	srv.sdkServerDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		attempts++
		if network != "tcp" {
			t.Fatalf("unexpected network: %s", network)
		}
		if address != "10.0.0.8:9757" {
			t.Fatalf("unexpected address: %s", address)
		}
		if attempts < 3 {
			return nil, newConnectionRefusedDialError()
		}
		return newReadySDKServerConn(t), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := srv.waitForSDKServerReachable(ctx, "http://10.0.0.8:9757"); err != nil {
		t.Fatalf("waitForSDKServerReachable failed: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestWaitForSDKServerReachableStopsOnNonRetriableError(t *testing.T) {
	srv := newTestAPIServerForSDKServer(t)

	attempts := 0
	srv.sdkServerDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		attempts++
		return nil, io.EOF
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := srv.waitForSDKServerReachable(ctx, "http://10.0.0.8:9757")
	if err == nil {
		t.Fatalf("expected waitForSDKServerReachable to fail")
	}
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestHandleExecDevboxWaitsForSDKServerReachability(t *testing.T) {
	srv := newSDKServerRouteTestServer(t)

	attempts := 0
	srv.sdkServerDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		attempts++
		if address != "10.0.0.8:9757" {
			t.Fatalf("unexpected address: %s", address)
		}
		if attempts == 1 {
			return nil, newConnectionRefusedDialError()
		}
		return newReadySDKServerConn(t), nil
	}

	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "http://10.0.0.8:9757/api/v1/process/exec-sync" {
			t.Fatalf("unexpected upstream URL: %s", req.URL.String())
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"status":0,"message":"success","stdout":"ok","stderr":"","exitCode":0}`)),
		}, nil
	})
	defer func() {
		http.DefaultClient.Transport = originalTransport
	}()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox/demo-devbox/exec",
		bytes.NewBufferString(`{"command":["pwd"],"timeoutSeconds":30}`),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if attempts != 2 {
		t.Fatalf("expected 2 dial attempts, got %d", attempts)
	}
}

func TestHandleUploadFileWaitsForSDKServerReachability(t *testing.T) {
	srv := newSDKServerRouteTestServer(t)

	attempts := 0
	srv.sdkServerDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		attempts++
		if address != "10.0.0.8:9757" {
			t.Fatalf("unexpected address: %s", address)
		}
		if attempts == 1 {
			return nil, newConnectionRefusedDialError()
		}
		return newReadySDKServerConn(t), nil
	}

	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "http://10.0.0.8:9757/api/v1/files/write?path=%2Ftmp%2Fhello.txt" {
			t.Fatalf("unexpected upstream URL: %s", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer sdk-token" {
			t.Fatalf("unexpected upstream token: %s", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read upload body failed: %v", err)
		}
		if string(body) != "hello" {
			t.Fatalf("unexpected upload body: %q", string(body))
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"status":0,"message":"success"}`)),
		}, nil
	})
	defer func() {
		http.DefaultClient.Transport = originalTransport
	}()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/devbox/demo-devbox/files/upload?path=/tmp/hello.txt",
		strings.NewReader("hello"),
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if attempts != 2 {
		t.Fatalf("expected 2 dial attempts, got %d", attempts)
	}
}

func TestHandleDownloadFileWaitsForSDKServerReachability(t *testing.T) {
	srv := newSDKServerRouteTestServer(t)

	attempts := 0
	srv.sdkServerDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		attempts++
		if address != "10.0.0.8:9757" {
			t.Fatalf("unexpected address: %s", address)
		}
		if attempts == 1 {
			return nil, newConnectionRefusedDialError()
		}
		return newReadySDKServerConn(t), nil
	}

	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "http://10.0.0.8:9757/api/v1/files/read?path=%2Ftmp%2Fhello.txt" {
			t.Fatalf("unexpected upstream URL: %s", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer sdk-token" {
			t.Fatalf("unexpected upstream token: %s", got)
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("hello")),
		}
		resp.Header.Set("Content-Type", "text/plain")
		resp.Header.Set("Content-Length", "5")
		return resp, nil
	})
	defer func() {
		http.DefaultClient.Transport = originalTransport
	}()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/devbox/demo-devbox/files/download?path=/tmp/hello.txt",
		nil,
	)
	req.Header.Set("Authorization", issueBearerTokenForNamespace(t, "ns-test"))
	resp := httptest.NewRecorder()
	srv.routes().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "hello" {
		t.Fatalf("unexpected download body: %q", resp.Body.String())
	}
	if attempts != 2 {
		t.Fatalf("expected 2 dial attempts, got %d", attempts)
	}
}

func TestLoadDevboxJWTSecretCache(t *testing.T) {
	srv := newTestAPIServerForSDKServer(
		t,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Data: map[string][]byte{
				devboxJWTSecretKey: []byte("token-v1"),
			},
		},
	)

	token, err := srv.loadDevboxJWTSecret(context.Background(), "ns-test", "demo-devbox")
	if err != nil {
		t.Fatalf("load token failed: %v", err)
	}
	if token != "token-v1" {
		t.Fatalf("unexpected token: %s", token)
	}

	updated := &corev1.Secret{}
	if err := srv.ctrlClient.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "ns-test", Name: "demo-devbox"}, updated); err != nil {
		t.Fatalf("get secret failed: %v", err)
	}
	updated.Data[devboxJWTSecretKey] = []byte("token-v2")
	if err := srv.ctrlClient.Update(context.Background(), updated); err != nil {
		t.Fatalf("update secret failed: %v", err)
	}

	cached, err := srv.loadDevboxJWTSecret(context.Background(), "ns-test", "demo-devbox")
	if err != nil {
		t.Fatalf("load cached token failed: %v", err)
	}
	if cached != "token-v1" {
		t.Fatalf("expected cache hit token-v1, got: %s", cached)
	}

	cacheKey := devboxJWTSecretCacheKey("ns-test", "demo-devbox")
	srv.jwtSecretCacheMu.Lock()
	entry := srv.jwtSecretCache[cacheKey]
	entry.expiresAt = time.Now().Add(-time.Second)
	srv.jwtSecretCache[cacheKey] = entry
	srv.jwtSecretCacheMu.Unlock()

	refreshed, err := srv.loadDevboxJWTSecret(context.Background(), "ns-test", "demo-devbox")
	if err != nil {
		t.Fatalf("load refreshed token failed: %v", err)
	}
	if refreshed != "token-v2" {
		t.Fatalf("expected refreshed token-v2, got: %s", refreshed)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newSDKServerRouteTestServer(t *testing.T) *apiServer {
	t.Helper()

	srv := newTestAPIServerForSDKServer(
		t,
		&devboxv1alpha2.Devbox{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
				UID:       "devbox-uid-v1",
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "demo-container"},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.0.0.8",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-devbox",
				Namespace: "ns-test",
			},
			Data: map[string][]byte{
				devboxJWTSecretKey: []byte("sdk-token"),
			},
		},
	)
	srv.cfg.JWTSigningKey = "test-secret"
	return srv
}

func newReadySDKServerConn(t *testing.T) net.Conn {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
	})
	return serverConn
}

func newConnectionRefusedDialError() error {
	return &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
	}
}

func newTestAPIServerForSDKServer(t *testing.T, objs ...ctrlclient.Object) *apiServer {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme failed: %v", err)
	}
	if err := devboxv1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("add devbox scheme failed: %v", err)
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	return &apiServer{
		ctrlClient: client,
		logger:     newLogger(io.Discard, slog.LevelDebug),
		sdkServerDialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return newReadySDKServerConn(t), nil
		},
	}
}
