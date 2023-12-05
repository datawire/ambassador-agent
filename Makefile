A8R_AGENT_VERSION ?= $(shell unset GOOS GOARCH; go run ./build-aux/genversion)
# Ensure that the variable is fully expanded. We don't want to call genversion repeatedly
# as it may produce different results every time.
A8R_AGENT_VERSION := ${A8R_AGENT_VERSION}
A8R_AGENT_REGISTRY ?= datawiredev
IMAGE_VERSION = $(patsubst v%,%,$(A8R_AGENT_VERSION))
IMAGE = ${A8R_AGENT_REGISTRY}/ambassador-agent:${IMAGE_VERSION}
BUILDDIR=build-output

include build-aux/tools.mk

# DOCKER_BUILDKIT is _required_ by our Dockerfile, since we use
# Dockerfile extensions for the Go build cache.  See
# https://github.com/moby/buildkit/blob/master/frontend/dockerfile/docs/syntax.md.
export DOCKER_BUILDKIT := 1

$(BUILDDIR)/go1%.src.tar.gz:
	mkdir -p $(BUILDDIR)
	curl -o $@ --fail -L https://dl.google.com/go/$(@F)

PKG_AGENT = $(shell go list ./pkg/agent)

.PHONY: build
build:
	mkdir -p $(BUILDDIR)/bin
	CGO_ENABLED=0 go build \
	-trimpath \
	-ldflags=-X=$(PKG_AGENT).Version=$(A8R_AGENT_VERSION) \
	-o=$(BUILDDIR)/bin/ambassador-agent \
	./cmd/main.go

.PHONY: format
format: $(tools/golangci-lint) ## (QA) Automatically fix linter complaints
	$(tools/golangci-lint) run -v --fix --timeout 2m ./... || true

lint: $(tools/golangci-lint) ## (QA) Run the linter
	$(tools/golangci-lint) run -v --timeout 8m ./...


.PHONY: protoc
protoc: ## (Protoc) Update .pb and .grpc.pb files that get checked in to Git from .proto files
protoc: protoc-director protoc-rpc

.PHONY: protoc-deps
protoc-deps: $(tools/protoc) $(tools/protoc-gen-go) $(tools/protoc-gen-go-grpc)

.PHONY: protoc-director
protoc-director: protoc-deps
	mkdir -p ./pkg/api
	$(tools/protoc) \
    	-I=./api \
    	--proto_path=./api \
    	--go_opt=paths=source_relative \
    	--go_out=./pkg/api \
    	--go-grpc_opt=paths=source_relative \
    	--go-grpc_out=./pkg/api \
    	$(shell find ./api -iname "*.proto")

.PHONY: protoc-rpc
protoc-rpc: protoc-deps
	$(tools/protoc) \
		-I rpc \
		--go_out=./rpc \
		--go_opt=module=github.com/datawire/ambassador-agent/rpc \
		--go-grpc_out=./rpc \
		--go-grpc_opt=module=github.com/datawire/ambassador-agent/rpc \
		--proto_path=. \
		$$(find ./rpc/ -name '*.proto')

.PHONY: generate
generate: ## (Generate) Update generated files that get checked in to Git
generate: generate-clean
generate: protoc $(tools/go-mkopensource) $(BUILDDIR)/$(shell go env GOVERSION).src.tar.gz
generate:
	cd ./rpc && export GOFLAGS=-mod=mod && go mod tidy && go mod vendor && rm -rf vendor

	export GOFLAGS=-mod=mod && go mod tidy && go mod vendor

	mkdir -p $(BUILDDIR)
	$(tools/go-mkopensource) --ignore-dirty --gotar=$(filter %.src.tar.gz,$^) --output-format=txt --package=mod --application-type=external  \
		--unparsable-packages build-aux/unparsable-packages.yaml >$(BUILDDIR)/DEPENDENCIES.txt
	sed 's/\(^.*the Go language standard library ."std".[ ]*v[1-9]\.[0-9]*\)\..../\1    /' $(BUILDDIR)/DEPENDENCIES.txt >DEPENDENCIES.md

	printf "ambassador-agent incorporates Free and Open Source software under the following licenses:\n\n" > DEPENDENCY_LICENSES.md
	$(tools/go-mkopensource) --ignore-dirty --gotar=$(filter %.src.tar.gz,$^) --output-format=txt --package=mod \
		--output-type=json --application-type=external --unparsable-packages build-aux/unparsable-packages.yaml > $(BUILDDIR)/DEPENDENCIES.json
	jq -r '.licenseInfo | to_entries | .[] | "* [" + .key + "](" + .value + ")"' $(BUILDDIR)/DEPENDENCIES.json > $(BUILDDIR)/LICENSES.txt
	sed -e 's/\[\([^]]*\)]()/\1/' $(BUILDDIR)/LICENSES.txt >> DEPENDENCY_LICENSES.md
	rm -rf vendor

.PHONY: generate-clean
generate-clean: ## (Generate) Delete generated files
	rm -f DEPENDENCIES.md
	rm -f DEPENDENCY_LICENSES.md

.PHONY: image
image:
	docker build --build-arg A8R_AGENT_VERSION=$(A8R_AGENT_VERSION) --tag $(IMAGE) .

.PHONY: push-ximage
push-ximage: image
	if docker pull $(IMAGE); then \
	  print "Failure: Tag already exists"; \
	  exit 1; \
	fi
	docker buildx build --build-arg A8R_AGENT_VERSION=$(A8R_AGENT_VERSION) --push --platform=linux/amd64,linux/arm64 --tag=$(IMAGE) .

.PHONY: push-image
push-image: image
	if docker pull $(IMAGE); then \
	  printf "Failure: Tag already exists\n"; \
	  exit 1; \
	fi
	docker build --build-arg A8R_AGENT_VERSION=$(A8R_AGENT_VERSION) --tag=$(IMAGE) .
	docker push $(IMAGE)

.PHONY: image-tar
image-tar: image
	mkdir -p ./build-output
	docker save $(IMAGE) > ./build-output/ambassador-agent-image.tar

.PHONY: go-integration-test
go-integration-test:
	go mod download
	go test -v -parallel 1 ./integration_tests/...

.PHONY: go-unit-test
go-unit-test:
	go mod download
	go test ./cmd/... -race
	go test ./pkg/... -race

.PHONY: apply
apply: push-image
	helm install ambassador-agent ./helm/ambassador-agent -n ambassador --set image.fullImageOverride=$(IMAGE) --set logLevel=DEBUG --set cloudConnectToken=$(APIKEY)

.PHONY: delete
delete:
	helm delete ambassador-agent -n ambassador