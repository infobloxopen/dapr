# ------------------------------------------------------------
# Copyright (c) Microsoft Corporation and Dapr Contributors.
# Licensed under the MIT License.
# ------------------------------------------------------------

################################################################################
# Variables                                                                    #
################################################################################

export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org
export GOSUMDB ?= sum.golang.org
##############srilakshmi#########################################################

PROJECT_ROOT := smanjunath2/dapr
REPO         := smanjunath2
GITHUB_REPO  := git@github.com:srilakshmimanjunath2





#################end srilakshmi##################################################
GIT_COMMIT  = $(shell git rev-list -1 HEAD)
GIT_VERSION = $(shell git describe --always --abbrev=7 --dirty)
# By default, disable CGO_ENABLED. See the details on https://golang.org/cmd/cgo
CGO         ?= 0
BINARIES    ?= daprd placement operator injector sentry
HA_MODE     ?= false
# Force in-memory log for placement
FORCE_INMEM ?= true

# Add latest tag if LATEST_RELEASE is true
LATEST_RELEASE ?=

PROTOC ?=protoc

ifdef REL_VERSION
	DAPR_VERSION := $(REL_VERSION)
else
	DAPR_VERSION := edge
endif

LOCAL_ARCH := $(shell uname -m)
ifeq ($(LOCAL_ARCH),x86_64)
	TARGET_ARCH_LOCAL=amd64
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 5),armv8)
	TARGET_ARCH_LOCAL=arm64
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 4),armv)
	TARGET_ARCH_LOCAL=arm
else
	TARGET_ARCH_LOCAL=amd64
endif
export GOARCH ?= $(TARGET_ARCH_LOCAL)

ifeq ($(GOARCH),amd64)
	LATEST_TAG=latest
else
	LATEST_TAG=latest-$(GOARCH)
endif

LOCAL_OS := $(shell uname)
ifeq ($(LOCAL_OS),Linux)
   TARGET_OS_LOCAL = linux
else ifeq ($(LOCAL_OS),Darwin)
   TARGET_OS_LOCAL = darwin
else
   TARGET_OS_LOCAL ?= windows
endif
export GOOS ?= $(TARGET_OS_LOCAL)

# Default docker container and e2e test targst.
TARGET_OS ?= linux
TARGET_ARCH ?= amd64

ifeq ($(GOOS),windows)
BINARY_EXT_LOCAL:=.exe
GOLANGCI_LINT:=golangci-lint.exe
export ARCHIVE_EXT = .zip
else
BINARY_EXT_LOCAL:=
GOLANGCI_LINT:=golangci-lint
export ARCHIVE_EXT = .tar.gz
endif

export BINARY_EXT ?= $(BINARY_EXT_LOCAL)

OUT_DIR := ./dist

# Helm template and install setting
HELM:=helm
RELEASE_NAME?=dapr
DAPR_NAMESPACE?=dapr-system
HELM_CHART_ROOT:=./charts
HELM_CHART_DIR:=$(HELM_CHART_ROOT)/dapr
HELM_OUT_DIR:=$(OUT_DIR)/install
HELM_MANIFEST_FILE:=$(HELM_OUT_DIR)/$(RELEASE_NAME).yaml
HELM_REGISTRY?=daprio.azurecr.io
##############################################################################
#   srilakshmi                                                               #
##############################################################################
# Docker image build and push setting
DOCKER:=docker
DAPR_SYSTEM_IMAGE_NAME=$(RELEASE_NAME)
DAPR_RUNTIME_IMAGE_NAME=daprd
DAPR_PLACEMENT_IMAGE_NAME=placement
DAPR_SENTRY_IMAGE_NAME=sentry

# build docker image for linux
BIN_PATH=$(OUT_DIR)/$(TARGET_OS)_$(TARGET_ARCH)

ifeq ($(TARGET_OS), windows)
  DOCKERFILE:=Dockerfile-windows
  BIN_PATH := $(BIN_PATH)/release
else ifeq ($(origin DEBUG), undefined)
  DOCKERFILE:=Dockerfile
  BIN_PATH := $(BIN_PATH)/release
else ifeq ($(DEBUG),0)
  DOCKERFILE:=Dockerfile
  BIN_PATH := $(BIN_PATH)/release
else
  DOCKERFILE:=Dockerfile-debug
  BIN_PATH := $(BIN_PATH)/debug
endif

ifeq ($(TARGET_ARCH),arm)
  DOCKER_IMAGE_PLATFORM:=$(TARGET_OS)/arm/v7
else ifeq ($(TARGET_ARCH),arm64)
  DOCKER_IMAGE_PLATFORM:=$(TARGET_OS)/arm64/v8
else
  DOCKER_IMAGE_PLATFORM:=$(TARGET_OS)/amd64
endif

# Supported docker image architecture
DOCKERMUTI_ARCH=linux-amd64 linux-arm linux-arm64 windows-amd64

################################################################################
# Target: docker-build, docker-push                                            #
################################################################################
# configuration for image names
USERNAME		:= $(USERNAME)
GIT_COMMIT		:= $(shell git describe --tags --dirty=-unsupported --always || echo pre-commit)
IMAGE_VERSION ?= $(GIT_COMMIT)-j$(BUILD_NUMBER)
TAG_LATEST    ?= latest
#BUILD_PATH		:= bin
DOCKERFILE_DIR := $(CURDIR)/docker
#BUILD_NUMBER       ?= 0



LINUX_BINS_OUT_DIR=$(OUT_DIR)/linux_$(GOARCH)
DOCKER_IMAGE_TAG=$(REPO)/$(DAPR_SYSTEM_IMAGE_NAME):$(IMAGE_VERSION)
DAPR_RUNTIME_DOCKER_IMAGE_TAG=$(REPO)/$(DAPR_RUNTIME_IMAGE_NAME):$(IMAGE_VERSION)
DAPR_PLACEMENT_DOCKER_IMAGE_TAG=$(REPO)/$(DAPR_PLACEMENT_IMAGE_NAME):$(IMAGE_VERSION)
DAPR_SENTRY_DOCKER_IMAGE_TAG=$(REPO)/$(DAPR_SENTRY_IMAGE_NAME):$(IMAGE_VERSION)

ifeq ($(LATEST_RELEASE),true)
DOCKER_IMAGE_LATEST_TAG=$(REPO)/$(DAPR_SYSTEM_IMAGE_NAME):$(LATEST_TAG)
DAPR_RUNTIME_DOCKER_IMAGE_LATEST_TAG=$(REPO)/$(DAPR_RUNTIME_IMAGE_NAME):$(LATEST_TAG)
DAPR_PLACEMENT_DOCKER_IMAGE_LATEST_TAG=$(REPO)/$(DAPR_PLACEMENT_IMAGE_NAME):$(LATEST_TAG)
DAPR_SENTRY_DOCKER_IMAGE_LATEST_TAG=$(REPO)/$(DAPR_SENTRY_IMAGE_NAME):$(LATEST_TAG)
endif

# configuration for server binary and image
#DAPR_SERVER_BINARY 		:= $(BUILD_PATH)/server
#SERVER_PATH		:= $(PROJECT_ROOT)/cmd/server
#SERVER_IMAGE		:= $(REPO)/atlas.onprem.health-collector
#SERVER_DOCKERFILE	:= $(DAPR_DOCKERFILE_PATH)/Dockerfile

# configuration for the protobuf gentool
#SRCROOT_ON_HOST		:= $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
#SRCROOT_IN_CONTAINER	:= /go/src/$(PROJECT_ROOT)
#DOCKER_RUNNER    	:= docker run -u `id -u`:`id -g` --rm
#DOCKER_RUNNER		+= -v $(SRCROOT_ON_HOST):$(SRCROOT_IN_CONTAINER)
#DOCKER_GENERATOR	:= infoblox/atlas-gentool:latest
#GENERATOR		:= $(DOCKER_RUNNER) $(DOCKER_GENERATOR

##############################################################################
#end srilakshmi                                                              #
##############################################################################

################################################################################
# Go build details                                                             #
################################################################################
BASE_PACKAGE_NAME := github.com/dapr/dapr

DEFAULT_LDFLAGS:=-X $(BASE_PACKAGE_NAME)/pkg/version.commit=$(GIT_VERSION) -X $(BASE_PACKAGE_NAME)/pkg/version.version=$(DAPR_VERSION)

ifeq ($(origin DEBUG), undefined)
  BUILDTYPE_DIR:=release
  LDFLAGS:="$(DEFAULT_LDFLAGS) -s -w"
else ifeq ($(DEBUG),0)
  BUILDTYPE_DIR:=release
  LDFLAGS:="$(DEFAULT_LDFLAGS) -s -w"
else
  BUILDTYPE_DIR:=debug
  GCFLAGS:=-gcflags="all=-N -l"
  LDFLAGS:="$(DEFAULT_LDFLAGS)"
  $(info Build with debugger information)
endif

DAPR_OUT_DIR := $(OUT_DIR)/$(GOOS)_$(GOARCH)/$(BUILDTYPE_DIR)
DAPR_LINUX_OUT_DIR := $(OUT_DIR)/linux_$(GOARCH)/$(BUILDTYPE_DIR)

################################################################################
# Target: build                                                                #
################################################################################
.PHONY: build
DAPR_BINS:=$(foreach ITEM,$(BINARIES),$(DAPR_OUT_DIR)/$(ITEM)$(BINARY_EXT))
build: $(DAPR_BINS)

# Generate builds for dapr binaries for the target
# Params:
# $(1): the binary name for the target
# $(2): the binary main directory
# $(3): the target os
# $(4): the target arch
# $(5): the output directory
define genBinariesForTarget
.PHONY: $(5)/$(1)
$(5)/$(1):
	CGO_ENABLED=$(CGO) GOOS=$(3) GOARCH=$(4) go build $(GCFLAGS) -ldflags=$(LDFLAGS) \
	-o $(5)/$(1) $(2)/;
endef

# Generate binary targets
$(foreach ITEM,$(BINARIES),$(eval $(call genBinariesForTarget,$(ITEM)$(BINARY_EXT),./cmd/$(ITEM),$(GOOS),$(GOARCH),$(DAPR_OUT_DIR))))

################################################################################
# Target: build-linux                                                          #
################################################################################
BUILD_LINUX_BINS:=$(foreach ITEM,$(BINARIES),$(DAPR_LINUX_OUT_DIR)/$(ITEM))
build-linux: $(BUILD_LINUX_BINS)

# Generate linux binaries targets to build linux docker image
ifneq ($(GOOS), linux)
$(foreach ITEM,$(BINARIES),$(eval $(call genBinariesForTarget,$(ITEM),./cmd/$(ITEM),linux,$(GOARCH),$(DAPR_LINUX_OUT_DIR))))
endif

################################################################################
# Target: archive                                                              #
################################################################################
ARCHIVE_OUT_DIR ?= $(DAPR_OUT_DIR)
ARCHIVE_FILE_EXTS:=$(foreach ITEM,$(BINARIES),archive-$(ITEM)$(ARCHIVE_EXT))

archive: $(ARCHIVE_FILE_EXTS)

# Generate archive files for each binary
# $(1): the binary name to be archived
# $(2): the archived file output directory
define genArchiveBinary
ifeq ($(GOOS),windows)
archive-$(1).zip:
	7z.exe a -tzip "$(2)\\$(1)_$(GOOS)_$(GOARCH)$(ARCHIVE_EXT)" "$(DAPR_OUT_DIR)\\$(1)$(BINARY_EXT)"
else
archive-$(1).tar.gz:
	tar czf "$(2)/$(1)_$(GOOS)_$(GOARCH)$(ARCHIVE_EXT)" -C "$(DAPR_OUT_DIR)" "$(1)$(BINARY_EXT)"
endif
endef

# Generate archive-*.[zip|tar.gz] targets
$(foreach ITEM,$(BINARIES),$(eval $(call genArchiveBinary,$(ITEM),$(ARCHIVE_OUT_DIR))))


################################################################################
# Target: manifest-gen                                                         #
################################################################################

# Generate helm chart manifest
manifest-gen: dapr.yaml

dapr.yaml: check-docker-env
	$(info Generating helm manifest $(HELM_MANIFEST_FILE)...)
	@mkdir -p $(HELM_OUT_DIR)
	$(HELM) template \
		--include-crds=true  --set global.ha.enabled=$(HA_MODE) --set dapr_config.dapr_config_chart_included=false --set-string global.tag=$(DAPR_TAG) --set-string global.registry=$(DAPR_REGISTRY) $(HELM_CHART_DIR) > $(HELM_MANIFEST_FILE)

################################################################################
# Target: upload-helmchart
################################################################################

# Upload helm charts to Helm Registry
upload-helmchart:
	export HELM_EXPERIMENTAL_OCI=1; \
	$(HELM) chart save ${HELM_CHART_ROOT}/${RELEASE_NAME} ${HELM_REGISTRY}/${HELM}/${RELEASE_NAME}:${DAPR_VERSION}; \
	$(HELM) chart push ${HELM_REGISTRY}/${HELM}/${RELEASE_NAME}:${DAPR_VERSION}

################################################################################
# Target: docker-deploy-k8s                                                    #
################################################################################

docker-deploy-k8s: check-docker-env check-arch
	$(info Deploying ${DAPR_REGISTRY}/${RELEASE_NAME}:${DAPR_TAG} to the current K8S context...)
	$(HELM) install \
		$(RELEASE_NAME) --namespace=$(DAPR_NAMESPACE) --wait --timeout 5m0s \
		--set global.ha.enabled=$(HA_MODE) --set-string global.tag=$(DAPR_TAG)-$(TARGET_OS)-$(TARGET_ARCH) \
		--set-string global.registry=$(DAPR_REGISTRY) --set global.logAsJson=true \
		--set global.daprControlPlaneOs=$(TARGET_OS) --set global.daprControlPlaneArch=$(TARGET_ARCH) \
		--set dapr_placement.logLevel=debug \
		--set dapr_placement.cluster.forceInMemoryLog=$(FORCE_INMEM) $(HELM_CHART_DIR)

################################################################################
# Target: archive                                                              #
################################################################################
release: build archive

################################################################################
# Target: test                                                                 #
################################################################################
.PHONY: test
test:
	go test ./pkg/... $(COVERAGE_OPTS)
	go test ./tests/...

################################################################################
# Target: lint                                                                 #
################################################################################
# Due to https://github.com/golangci/golangci-lint/issues/580, we need to add --fix for windows
.PHONY: lint
lint:
	$(GOLANGCI_LINT) run --timeout=20m

#############srilakshmi#######################################################
.PHONY: docker
docker:
	$(info Building $(DOCKER_IMAGE_TAG) docker image ...)
	ifeq ($(TARGET_ARCH),amd64)
	      @docker build --build-arg PKG_FILES=* -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	      @docker build --build-arg PKG_FILES=daprd -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_RUNTIME_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	      @docker build --build-arg PKG_FILES=placement -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	      @docker build --build-arg PKG_FILES=sentry -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_SENTRY_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)             
	else
	     -@docker buildx create --use --name daprbuild
	     -@docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
	     @docker buildx build --build-arg PKG_FILES=* --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	     @docker buildx build --build-arg PKG_FILES=daprd --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_RUNTIME_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	     @docker buildx build --build-arg PKG_FILES=placement --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	     @docker buildx build --build-arg PKG_FILES=sentry --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_SENTRY_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)

        endif
              
##################end srilakshmi##############################################

################################################################################
# Target: modtidy                                                              #
################################################################################
.PHONY: modtidy
modtidy:
	go mod tidy

################################################################################
# Target: init-proto                                                            #
################################################################################
.PHONY: init-proto
init-proto:
	go get google.golang.org/protobuf/cmd/protoc-gen-go@v1.25.0 google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1.0


###########srilakshmi###########################################################
#.PHONY: clean
#clean:
#	@docker rmi -f $(shell docker images -q $(SERVER_IMAGE)) || true

.PHONY: push
push: docker
	@docker push $(DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	@docker push $(DAPR_RUNTIME_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	@docker push $(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	@docker push $(DAPR_SENTRY_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)








#################end srilakshmi##################################################

################################################################################
# Target: gen-proto                                                            #
################################################################################
GRPC_PROTOS:=common internals operator placement runtime sentry
PROTO_PREFIX:=github.com/dapr/dapr

# Generate archive files for each binary
# $(1): the binary name to be archived
define genProtoc
.PHONY: gen-proto-$(1)
gen-proto-$(1):
	$(PROTOC) --go_out=. --go_opt=module=$(PROTO_PREFIX) --go-grpc_out=. --go-grpc_opt=require_unimplemented_servers=false,module=$(PROTO_PREFIX) ./dapr/proto/$(1)/v1/*.proto
endef

$(foreach ITEM,$(GRPC_PROTOS),$(eval $(call genProtoc,$(ITEM))))

GEN_PROTOS:=$(foreach ITEM,$(GRPC_PROTOS),gen-proto-$(ITEM))

.PHONY: gen-proto
gen-proto: $(GEN_PROTOS) modtidy

################################################################################
# Target: get-components-contrib                                               #
################################################################################
.PHONY: get-components-contrib
get-components-contrib:
	go get github.com/dapr/components-contrib@master

################################################################################
# Target: check-diff                                                           #
################################################################################
.PHONY: check-diff
check-diff:
	git diff --exit-code ./go.mod # check no changes
	git diff --exit-code ./go.sum # check no changes

################################################################################
# Target: codegen                                                              #
################################################################################
include tools/codegen.mk

################################################################################
# Target: docker                                                               #
################################################################################
#include docker/docker.mk

################################################################################
# Target: tests                                                                #
################################################################################
include tests/dapr_tests.mk
