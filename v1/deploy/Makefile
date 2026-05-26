SEALOS ?= sealos
DOCKER ?= docker
ARCH ?= amd64
IMAGE ?= devbox-v1-cluster:latest

.PHONY: all
all: lint

.PHONY: sync-crds
sync-crds:
	./scripts/sync-crds.sh

.PHONY: lint
lint: sync-crds
	helm lint charts/devbox-v1
	helm template devbox-v1 charts/devbox-v1 --namespace devbox-system --include-crds >/tmp/devbox-v1-rendered.yaml
	@! awk '/^---/ { in_namespace = 0 } /^kind: Namespace$$/ { in_namespace = 1; next } in_namespace && /^[[:space:]]*name:[[:space:]]*"?devbox-system"?[[:space:]]*$$/ { found = 1 } END { exit found ? 0 : 1 }' /tmp/devbox-v1-rendered.yaml || (echo "release namespace devbox-system must not be rendered as a chart Namespace" >&2; exit 1)
	helm template devbox-v1 charts/devbox-v1 --namespace devbox-system --set-string registry.addr=hub.example.test --set-string registry.user=test-user --set-string registry.password=test-password >/tmp/devbox-v1-registry-rendered.yaml
	@grep -q -- '--registry-addr=hub.example.test' /tmp/devbox-v1-registry-rendered.yaml
	@grep -q -- 'value: "hub.example.test"' /tmp/devbox-v1-registry-rendered.yaml

.PHONY: registry-save
registry-save: sync-crds
	$(SEALOS) registry save --registry-dir=registry_$(ARCH) --arch $(ARCH) .

.PHONY: docker-build
docker-build:
	$(DOCKER) buildx build --platform linux/$(ARCH) -f Kubefile -t $(IMAGE)-$(ARCH) .
