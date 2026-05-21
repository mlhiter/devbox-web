package commit

import (
	"fmt"

	"github.com/containerd/nerdctl/v2/pkg/imgutil/dockerconfigresolver"
)

// registerRegistryCredentials writes registry credentials to the nerdctl/docker config store.
// nerdctl login fails for HTTPS registries on port 443 when the registry omits the port in
// WWW-Authenticate (acArg host vs host:443 mismatch); storing credentials directly avoids that.
func registerRegistryCredentials(registryAddr, username, password string) error {
	registryURL, err := dockerconfigresolver.Parse(registryAddr)
	if err != nil {
		return err
	}

	credStore, err := dockerconfigresolver.NewCredentialsStore("")
	if err != nil {
		return err
	}

	credentials := &dockerconfigresolver.Credentials{
		Username: username,
		Password: password,
	}
	if err := credStore.Store(registryURL, credentials); err != nil {
		return fmt.Errorf("save registry credentials: %w", err)
	}

	// Match nerdctl login: also store without explicit :443 for default HTTPS port.
	if registryURL.Port() == dockerconfigresolver.StandardHTTPSPort {
		registryURL.Host = registryURL.Hostname()
		if err := credStore.Store(registryURL, credentials); err != nil {
			return fmt.Errorf("save registry credentials (host without port): %w", err)
		}
	}

	return nil
}
