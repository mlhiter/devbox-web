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
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// todo: refactor this struct, add opts for tls or something else
type Opts struct{}

type BasicAuth struct {
	Username string
	Password string
}

type Registry struct {
	Host      string
	BasicAuth BasicAuth
	Insecure  bool
}

var ErrManifestNotFound = errors.New("manifest not found")

// ReTag creates a new tag for an existing image by copying its manifest.
func (c *Registry) ReTag(source, target string) error {
	sourceRef, err := name.ParseReference(source, c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse source image: %w", err)
	}
	targetRef, err := name.NewTag(target, c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse target image: %w", err)
	}

	descriptor, err := remote.Get(sourceRef, c.remoteOptions()...)
	if err != nil {
		if isManifestNotFound(err) {
			return ErrManifestNotFound
		}
		return fmt.Errorf("failed to pull manifest for %s: %w", source, err)
	}
	if err := remote.Tag(targetRef, descriptor, c.remoteOptions()...); err != nil {
		return fmt.Errorf("failed to push manifest for %s: %w", target, err)
	}
	return nil
}

func (c *Registry) remoteOptions() []remote.Option {
	auth := authn.Anonymous
	if c.BasicAuth.Username != "" || c.BasicAuth.Password != "" {
		auth = &authn.Basic{
			Username: c.BasicAuth.Username,
			Password: c.BasicAuth.Password,
		}
	}
	opts := []remote.Option{remote.WithAuth(auth)}
	if c.Insecure {
		opts = append(opts, remote.WithTransport(insecureTransport()))
	}
	return opts
}

func (c *Registry) nameOptions() []name.Option {
	if c.Insecure {
		return []name.Option{name.Insecure}
	}
	return nil
}

func insecureTransport() http.RoundTripper {
	if transport, ok := remote.DefaultTransport.(*http.Transport); ok {
		clone := transport.Clone()
		if clone.TLSClientConfig == nil {
			clone.TLSClientConfig = &tls.Config{}
		} else {
			clone.TLSClientConfig = clone.TLSClientConfig.Clone()
		}
		clone.TLSClientConfig.InsecureSkipVerify = true
		return clone
	}
	return remote.DefaultTransport
}

func isManifestNotFound(err error) bool {
	var transportErr *transport.Error
	if errors.As(err, &transportErr) {
		return transportErr.StatusCode == http.StatusNotFound
	}
	return false
}
