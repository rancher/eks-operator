TARGETS := $(shell ls scripts)

ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(abspath $(ROOT_DIR)/bin)
GO_INSTALL = ./scripts/go_install.sh

MOCKGEN_VER := v1.6.0
MOCKGEN_BIN := mockgen
MOCKGEN := $(TOOLS_BIN_DIR)/$(MOCKGEN_BIN)-$(MOCKGEN_VER)

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

.PHONY: $(TARGETS)
$(TARGETS): .dapper
	./.dapper $@

$(MOCKGEN): ## Build mockgen from tools folder.
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/golang/mock/mockgen $(MOCKGEN_BIN) $(MOCKGEN_VER)

.PHONY: operator
operator:
	go build -o bin/eks-operator main.go

.PHONY: generate-go
generate-go: $(MOCKGEN)
	go generate ./pkg/eks/...

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	rm -rf build bin dist
