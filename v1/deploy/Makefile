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
	helm template devbox-v1 charts/devbox-v1 --include-crds >/tmp/devbox-v1-rendered.yaml

.PHONY: registry-save
registry-save: sync-crds
	$(SEALOS) registry save --registry-dir=registry_$(ARCH) --arch $(ARCH) .

.PHONY: docker-build
docker-build:
	$(DOCKER) buildx build --platform linux/$(ARCH) -f Kubefile -t $(IMAGE)-$(ARCH) .
