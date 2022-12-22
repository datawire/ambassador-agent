# Copyright 2020-2022 Datawire.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This file deals with installing programs used by the build.

TOOLSDIR=tools
TOOLSBINDIR=$(TOOLSDIR)/bin
TOOLSSRCDIR=$(TOOLSDIR)/src

GOHOSTOS ?= $(shell go env GOHOSTOS)
GOHOSTARCH ?= $(shell go env GOHOSTARCH)

# GOARCH defaults to GOHOSTARCH but can also be set by the caller of make.
GOARCH?=$(GOHOSTARCH)

export PATH := $(abspath $(TOOLSBINDIR)):$(PATH)

clobber: clobber-tools

.PHONY: clobber-tools
clobber-tools:
	rm -rf $(TOOLSBINDIR) $(TOOLSDIR)/include $(TOOLSDIR)/*.*


# Protobuf compiler
# =================
#
# Install protoc under $TOOLSDIR. A protoc that is already installed locally
# cannot be trusted since this must be the exact same version as used when
# running CI. If it isn't, the generate-check will fail.
tools/protoc = $(TOOLSBINDIR)/protoc
PROTOC_VERSION=3.17.3
PROTOC_ZIP=protoc-$(PROTOC_VERSION)-$(subst darwin,osx,$(GOHOSTOS))-$(shell uname -m).zip
$(TOOLSDIR)/$(PROTOC_ZIP):
	mkdir -p $(@D)
	curl -sfL https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/$(PROTOC_ZIP) -o $@
%/bin/protoc %/include %/readme.txt: %/$(PROTOC_ZIP)
	cd $* && unzip -q -o -DD $(<F)


# `go get`-able things
# ====================
#
# Install the all under $TOOLSDIR. Versions that are already in $GOBIN
# cannot be trusted since this must be the exact same version as used
# when running CI. If it isn't the generate-check will fail.
#
# Instead of having "VERSION" variables here, the versions are
# controlled by `tools/src/${thing}/go.mod` files.  Having those in
# separate per-tool go.mod files avoids conflicts between tools and
# avoid them polluting our main go.mod file.
tools/protoc-gen-go      = $(TOOLSBINDIR)/protoc-gen-go
tools/protoc-gen-go-grpc = $(TOOLSBINDIR)/protoc-gen-go-grpc
tools/golangci-lint      = $(TOOLSBINDIR)/golangci-lint
tools/go-mkopensource    = $(TOOLSBINDIR)/go-mkopensource
$(TOOLSBINDIR)/%: $(TOOLSSRCDIR)/%/go.mod $(TOOLSSRCDIR)/%/pin.go
	cd $(<D) && GOOS= GOARCH= go build -o $(abspath $@) $$(sed -En 's,^import "(.*)".*,\1,p' pin.go)