TARGETS := $(shell ls scripts)
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(abspath $(ROOT_DIR)/bin)
GO_INSTALL = ./scripts/go_install.sh
CLUSTER_NAME?="eks-operator-e2e"
GIT_BRANCH?=$(shell git branch --show-current)
GIT_COMMIT?=$(shell git rev-parse HEAD)
GIT_COMMIT_SHORT?=$(shell git rev-parse --short HEAD)
GIT_TAG?=v0.0.0
ifneq ($(GIT_BRANCH), main)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "v0.0.0" )
endif
TAG?=${GIT_TAG}-${GIT_COMMIT_SHORT}
REPO?=docker.io/rancher
IMAGE = $(REPO)/eks-operator:$(TAG)
MACHINE := rancher
# Define the target platforms that can be used across the ecosystem.
# Note that what would actually be used for a given project will be
# defined in TARGET_PLATFORMS, and must be a subset of the below:
DEFAULT_PLATFORMS := linux/amd64,linux/arm64,darwin/arm64,darwin/amd64
TARGET_PLATFORMS := linux/amd64,linux/arm64
BUILDX_ARGS ?= --sbom=true --attest type=provenance,mode=max

E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/config.yaml
CHART_VERSION?=$(subst v,,$(GIT_TAG))
RAWCOMMITDATE=$(shell git log -n1 --format="%at")
OPERATOR_CHART?=$(shell find $(ROOT_DIR) -type f -name "rancher-eks-operator-[0-9]*.tgz" -print)
CRD_CHART?=$(shell find $(ROOT_DIR) -type f -name "rancher-eks-operator-crd*.tgz" -print)

ifeq ($(shell go env GOOS),darwin) # Use the darwin/amd64 binary until an arm64 version is available
	COMMITDATE?=$(shell gdate -d @"${RAWCOMMITDATE}" "+%FT%TZ")
else
	COMMITDATE?=$(shell date -d @"${RAWCOMMITDATE}" "+%FT%TZ")
endif

MOCKGEN_VER := v1.6.0
MOCKGEN_BIN := mockgen
MOCKGEN := $(BIN_DIR)/$(MOCKGEN_BIN)-$(MOCKGEN_VER)

GINKGO_VER := v2.20.2
GINKGO_BIN := ginkgo
GINKGO := $(BIN_DIR)/$(GINKGO_BIN)-$(GINKGO_VER)

GO_APIDIFF_VER := v0.8.2
GO_APIDIFF_BIN := go-apidiff
GO_APIDIFF := $(BIN_DIR)/$(GO_APIDIFF_BIN)-$(GO_APIDIFF_VER)

SETUP_ENVTEST_VER := v0.0.0-20211110210527-619e6b92dab9
SETUP_ENVTEST_BIN := setup-envtest
SETUP_ENVTEST := $(BIN_DIR)/$(SETUP_ENVTEST_BIN)-$(SETUP_ENVTEST_VER)

ifeq ($(shell go env GOOS),darwin) # Use the darwin/amd64 binary until an arm64 version is available
	KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path --arch amd64 $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))
else
	KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))
endif

default: operator

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

.PHONY: $(TARGETS)
$(TARGETS): .dapper
	./.dapper $@

$(MOCKGEN):
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/golang/mock/mockgen $(MOCKGEN_BIN) $(MOCKGEN_VER)

$(GINKGO):
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/onsi/ginkgo/v2/ginkgo $(GINKGO_BIN) $(GINKGO_VER)

$(GO_APIDIFF):
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/joelanford/go-apidiff $(GO_APIDIFF_BIN) $(GO_APIDIFF_VER)

$(SETUP_ENVTEST): 
	GOBIN=$(BIN_DIR) $(GO_INSTALL) sigs.k8s.io/controller-runtime/tools/setup-envtest $(SETUP_ENVTEST_BIN) $(SETUP_ENVTEST_VER)

.PHONY: operator
operator:
	CGO_ENABLED=0 go build -ldflags \
            "-X github.com/rancher/eks-operator/pkg/version.GitCommit=$(GIT_COMMIT) \
             -X github.com/rancher/eks-operator/pkg/version.Version=$(TAG)" \
        -o bin/eks-operator .

.PHONY: generate-go
generate-go: $(MOCKGEN)
	go generate ./pkg/eks/...

.PHONY: generate-crd
generate-crd: $(MOCKGEN)
	go generate main.go

.PHONY: generate
generate:
	$(MAKE) generate-go
	$(MAKE) generate-crd

buildx-machine: ## create rancher dockerbuildx machine targeting platform defined by DEFAULT_PLATFORMS
	@docker buildx ls | grep $(MACHINE) || \
		docker buildx create --name=$(MACHINE) --platform=$(DEFAULT_PLATFORMS)

.PHONY: image-build
image-build: buildx-machine ## build (and load) the container image targeting the current platform.
	docker buildx build -f package/Dockerfile \
    --builder $(MACHINE) --build-arg COMMIT=$(GIT_COMMIT) --build-arg VERSION=$(TAG) \
    -t "$(IMAGE)" $(BUILD_ACTION) .
	@echo "Built $(IMAGE)"

.PHONY: image-push
image-push: buildx-machine ## build the container image targeting all platforms defined by TARGET_PLATFORMS and push to a registry.
	docker buildx build -f package/Dockerfile \
    --builder $(MACHINE) $(IID_FILE_FLAG) $(BUILDX_ARGS) --build-arg COMMIT=$(GIT_COMMIT) --build-arg VERSION=$(TAG) \
    --platform=$(TARGET_PLATFORMS) -t "$(IMAGE)" --push .
	@echo "Pushed $(IMAGE)"

ALL_VERIFY_CHECKS = generate

.PHONY: verify
verify: $(addprefix verify-,$(ALL_VERIFY_CHECKS))

.PHONY: verify-generate
verify-generate: generate
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: test
test: $(SETUP_ENVTEST) $(GINKGO)
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GINKGO) -v -r -p --trace ./pkg/... ./controller/...

.PHONY: clean
clean:
	rm -rf build bin dist

.PHONY: operator-chart
operator-chart:
	mkdir -p $(BIN_DIR)
	cp -rf $(ROOT_DIR)/charts/eks-operator $(BIN_DIR)/chart
	sed -i -e 's/tag:.*/tag: '${TAG}'/' $(BIN_DIR)/chart/values.yaml
	sed -i -e 's|repository:.*|repository: '${REPO}/eks-operator'|' $(BIN_DIR)/chart/values.yaml
	helm package --version ${CHART_VERSION} --app-version ${GIT_TAG} -d $(BIN_DIR)/ $(BIN_DIR)/chart
	rm -Rf $(BIN_DIR)/chart
	
.PHONY: crd-chart
crd-chart:
	mkdir -p $(BIN_DIR)
	helm package --version ${CHART_VERSION} --app-version ${GIT_TAG} -d $(BIN_DIR)/ $(ROOT_DIR)/charts/eks-operator-crd
	rm -Rf $(BIN_DIR)/chart

.PHONY: charts
charts:
	$(MAKE) operator-chart
	$(MAKE) crd-chart

.PHONY: setup-kind
setup-kind:
	CLUSTER_NAME=$(CLUSTER_NAME) $(ROOT_DIR)/scripts/setup-kind-cluster.sh

.PHONY: e2e-tests
e2e-tests: $(GINKGO) charts
	export EXTERNAL_IP=`kubectl get nodes -o jsonpath='{.items[].status.addresses[?(@.type == "InternalIP")].address}'` && \
	export BRIDGE_IP="172.18.0.1" && \
	export CONFIG_PATH=$(E2E_CONF_FILE) && \
	export OPERATOR_CHART=$(OPERATOR_CHART) && \
	export CRD_CHART=$(CRD_CHART) && \
	cd $(ROOT_DIR)/test && $(GINKGO) $(ONLY_DEPLOY) -r -v ./e2e

.PHONY: kind-e2e-tests
kind-e2e-tests: docker-build-e2e setup-kind
	kind load docker-image --name $(CLUSTER_NAME) ${IMAGE}
	$(MAKE) e2e-tests

kind-deploy-operator:
	ONLY_DEPLOY="--label-filter=\"do-nothing\"" $(MAKE) kind-e2e-tests

.PHONY: docker-build-e2e
docker-build-e2e:
	DOCKER_BUILDKIT=1 docker build \
		-f test/e2e/Dockerfile.e2e \
		--build-arg "TAG=${GIT_TAG}" \
		--build-arg "COMMIT=${GIT_COMMIT}" \
		--build-arg "COMMITDATE=${COMMITDATE}" \
		-t ${IMAGE} .

.PHOHY: delete-local-kind-cluster
delete-local-kind-cluster: ## Delete the local kind cluster
	kind delete cluster --name=$(CLUSTER_NAME)

APIDIFF_OLD_COMMIT ?= $(shell git rev-parse origin/release-v2.10)

.PHONY: apidiff
apidiff: $(GO_APIDIFF) ## Check for API differences
	$(GO_APIDIFF) $(APIDIFF_OLD_COMMIT) --print-compatible
