// Copyright © 2024 sealos.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestRegistryReTag(t *testing.T) {
	const (
		username = "admin"
		password = "passw0rd"
		source   = "example.test/default/devbox-sample:old-tag"
		target   = "example.test/default/devbox-sample:new-tag"
	)

	var gotPutBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != username || pass != password {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/default/devbox-sample/manifests/old-tag":
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			_, _ = w.Write([]byte(`{"schemaVersion":2}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v2/default/devbox-sample/manifests/new-tag":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			gotPutBody = string(body)
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	restoreDefaultTransport := replaceDefaultTransport(server.Client().Transport)
	defer restoreDefaultTransport()

	registry := &Registry{
		BasicAuth: BasicAuth{
			Username: username,
			Password: password,
		},
	}

	sourceImage := strings.Replace(source, "example.test", strings.TrimPrefix(server.URL, "http://"), 1)
	targetImage := strings.Replace(target, "example.test", strings.TrimPrefix(server.URL, "http://"), 1)

	if err := registry.ReTag(sourceImage, targetImage); err != nil {
		t.Fatalf("ReTag() error = %v, want nil", err)
	}

	if gotPutBody != `{"schemaVersion":2}` {
		t.Fatalf("ReTag() pushed manifest = %q, want %q", gotPutBody, `{"schemaVersion":2}`)
	}
}

func TestRegistryReTagWithBearerChallenge(t *testing.T) {
	const (
		username = "admin"
		password = "passw0rd"
		token    = "token-for-70-style-registry"
	)

	var (
		gotTokenRequest bool
		gotPutBody      string
		server          *httptest.Server
	)
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/":
			w.Header().Set(
				"WWW-Authenticate",
				`Bearer realm="`+server.URL+`/token",service="`+r.Host+`"`,
			)
			w.WriteHeader(http.StatusUnauthorized)
		case r.Method == http.MethodGet && r.URL.Path == "/token":
			gotTokenRequest = true
			if user, pass, ok := r.BasicAuth(); !ok || user != username || pass != password {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
		case r.Method == http.MethodGet &&
			r.URL.Path == "/v2/default/devbox-sample/manifests/old-tag":
			if got := r.Header.Get("Authorization"); got != "Bearer "+token {
				t.Fatalf("GET Authorization = %q, want bearer token", got)
			}
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			_, _ = w.Write([]byte(`{"schemaVersion":2}`))
		case r.Method == http.MethodPut &&
			r.URL.Path == "/v2/default/devbox-sample/manifests/new-tag":
			if got := r.Header.Get("Authorization"); got != "Bearer "+token {
				t.Fatalf("PUT Authorization = %q, want bearer token", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			gotPutBody = string(body)
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	restoreDefaultTransport := replaceDefaultTransport(server.Client().Transport)
	defer restoreDefaultTransport()

	registry := &Registry{
		BasicAuth: BasicAuth{
			Username: username,
			Password: password,
		},
	}

	sourceImage := strings.TrimPrefix(server.URL, "https://") + "/default/devbox-sample:old-tag"
	targetImage := strings.TrimPrefix(server.URL, "https://") + "/default/devbox-sample:new-tag"

	if err := registry.ReTag(sourceImage, targetImage); err != nil {
		t.Fatalf("ReTag() error = %v, want nil", err)
	}

	if !gotTokenRequest {
		t.Fatal("ReTag() did not request a bearer token")
	}
	if gotPutBody != `{"schemaVersion":2}` {
		t.Fatalf("ReTag() pushed manifest = %q, want %q", gotPutBody, `{"schemaVersion":2}`)
	}
}

func TestRegistryReTagWithInsecureHTTPRegistryAndHTTPSTokenRealm(t *testing.T) {
	const (
		username = "admin"
		password = "passw0rd"
		token    = "token-for-internal-registry"
	)

	tokenServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if user, pass, ok := r.BasicAuth(); !ok || user != username || pass != password {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer tokenServer.Close()

	var gotPutBody string
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/":
			w.Header().Set(
				"WWW-Authenticate",
				`Bearer realm="`+tokenServer.URL+`/token",service="`+r.Host+`"`,
			)
			w.WriteHeader(http.StatusUnauthorized)
		case r.Method == http.MethodGet &&
			r.URL.Path == "/v2/default/devbox-sample/manifests/old-tag":
			if got := r.Header.Get("Authorization"); got != "Bearer "+token {
				t.Fatalf("GET Authorization = %q, want bearer token", got)
			}
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			_, _ = w.Write([]byte(`{"schemaVersion":2}`))
		case r.Method == http.MethodPut &&
			r.URL.Path == "/v2/default/devbox-sample/manifests/new-tag":
			if got := r.Header.Get("Authorization"); got != "Bearer "+token {
				t.Fatalf("PUT Authorization = %q, want bearer token", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			gotPutBody = string(body)
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer registryServer.Close()

	registry := &Registry{
		BasicAuth: BasicAuth{
			Username: username,
			Password: password,
		},
		Insecure: true,
	}

	sourceImage := strings.TrimPrefix(registryServer.URL, "http://") + "/default/devbox-sample:old-tag"
	targetImage := strings.TrimPrefix(registryServer.URL, "http://") + "/default/devbox-sample:new-tag"

	if err := registry.ReTag(sourceImage, targetImage); err != nil {
		t.Fatalf("ReTag() error = %v, want nil", err)
	}

	if gotPutBody != `{"schemaVersion":2}` {
		t.Fatalf("ReTag() pushed manifest = %q, want %q", gotPutBody, `{"schemaVersion":2}`)
	}
}

func TestRegistryPullManifestNotFound(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodGet &&
			r.URL.Path == "/v2/default/devbox-sample/manifests/missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	restoreDefaultTransport := replaceDefaultTransport(server.Client().Transport)
	defer restoreDefaultTransport()

	registry := &Registry{}
	sourceImage := strings.TrimPrefix(server.URL, "https://") + "/default/devbox-sample:missing"
	targetImage := strings.TrimPrefix(server.URL, "https://") + "/default/devbox-sample:new-tag"

	err := registry.ReTag(sourceImage, targetImage)
	if err != ErrManifestNotFound {
		t.Fatalf("ReTag() error = %v, want %v", err, ErrManifestNotFound)
	}
}

func replaceDefaultTransport(transport http.RoundTripper) func() {
	original := remote.DefaultTransport
	remote.DefaultTransport = transport
	return func() {
		remote.DefaultTransport = original
	}
}
