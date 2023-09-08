TARGETS := $(shell ls scripts)

ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(abspath $(ROOT_DIR)/bin)
GO_INSTALL = ./scripts/go_install.sh
CLUSTER_NAME?="eks-operator-e2e"
GIT_COMMIT?=$(shell git rev-parse HEAD)
GIT_COMMIT_SHORT?=$(shell git rev-parse --short HEAD)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "v0.0.0" )
TAG?=${GIT_TAG}-${GIT_COMMIT_SHORT}
REPO?=ghcr.io/rancher/eks-operator
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

GINKGO_VER := v2.12.0
GINKGO_BIN := ginkgo
GINKGO := $(BIN_DIR)/$(GINKGO_BIN)-$(GINKGO_VER)

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

.PHONY: operator
operator:
	go build -o bin/eks-operator main.go

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
test: $(GINKGO)
	$(GINKGO) -v -r --trace --race ./pkg/... ./controller/...

.PHONY: clean
clean:
	rm -rf build bin dist

.PHONY: operator-chart
operator-chart:
	mkdir -p $(BIN_DIR)
	cp -rf $(ROOT_DIR)/charts/eks-operator $(BIN_DIR)/chart
	sed -i -e 's/tag:.*/tag: '${TAG}'/' $(BIN_DIR)/chart/values.yaml
	sed -i -e 's|repository:.*|repository: '${REPO}'|' $(BIN_DIR)/chart/values.yaml
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
	$(ROOT_DIR)/scripts/setup-kind-cluster.sh

.PHONY: e2e-tests
e2e-tests: $(GINKGO) charts
	export EXTERNAL_IP=`kubectl get nodes -o jsonpath='{.items[].status.addresses[?(@.type == "InternalIP")].address}'` && \
	export BRIDGE_IP="172.18.0.1" && \
	export CONFIG_PATH=$(E2E_CONF_FILE) && \
	export OPERATOR_CHART=$(OPERATOR_CHART) && \
	export CRD_CHART=$(CRD_CHART) && \
	cd $(ROOT_DIR)/test && $(GINKGO) -r -v ./e2e

.PHONY: kind-e2e-tests
kind-e2e-tests: docker-build-e2e setup-kind
	kind load docker-image --name $(CLUSTER_NAME) ${REPO}:${TAG}
	$(MAKE) e2e-tests

.PHONY: docker-build-e2e
docker-build-e2e:
	DOCKER_BUILDKIT=1 docker build \
		-f test/e2e/Dockerfile.e2e \
		--build-arg "TAG=${GIT_TAG}" \
		--build-arg "COMMIT=${GIT_COMMIT}" \
		--build-arg "COMMITDATE=${COMMITDATE}" \
		-t ${REPO}:${TAG} .
