A8R_AGENT_VERSION ?= dev-latest
DEV_REGISTRY ?= datawiredev
IMAGE = ${DEV_REGISTRY}/ambassador-agent:${A8R_AGENT_VERSION}
BUILDDIR=build-output

include build-aux/tools.mk

# DOCKER_BUILDKIT is _required_ by our Dockerfile, since we use
# Dockerfile extensions for the Go build cache.  See
# https://github.com/moby/buildkit/blob/master/frontend/dockerfile/docs/syntax.md.
export DOCKER_BUILDKIT := 1

$(BUILDDIR)/go1%.src.tar.gz:
	mkdir -p $(BUILDDIR)
	curl -o $@ --fail -L https://dl.google.com/go/$(@F)

.PHONY: build
build:
	mkdir -p $(BUILDDIR)/bin
	CGO_ENABLED=0 go build \
	-trimpath \
	-ldflags=-X=main.version=${A8R_AGENT_VERSION} \
	-o=$(BUILDDIR)/bin/ambassador-agent \
	./cmd/main.go

.PHONY: format
format: $(tools/golangci-lint) ## (QA) Automatically fix linter complaints
	$(tools/golangci-lint) run --fix --timeout 2m ./... || true

lint: $(tools/golangci-lint) ## (QA) Run the linter
	$(tools/golangci-lint) run --timeout 8m ./...


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
	$(tools/go-mkopensource) --gotar=$(filter %.src.tar.gz,$^) --output-format=txt --package=mod --application-type=external  \
		--unparsable-packages build-aux/unparsable-packages.yaml >$(BUILDDIR)/DEPENDENCIES.txt
	sed 's/\(^.*the Go language standard library ."std".[ ]*v[1-9]\.[1-9]*\)\..../\1    /' $(BUILDDIR)/DEPENDENCIES.txt >DEPENDENCIES.md

	printf "ambassador-agent incorporates Free and Open Source software under the following licenses:\n\n" > DEPENDENCY_LICENSES.md
	$(tools/go-mkopensource) --gotar=$(filter %.src.tar.gz,$^) --output-format=txt --package=mod \
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
	docker build --tag $(IMAGE) .

.PHONY: image-push
image-push: image
	docker push $(IMAGE)

.PHONY: image-tar
image-tar: image
	mkdir -p ./build-output
	docker save $(IMAGE) > ./build-output/ambassador-agent-image.tar

.PHONY: itest
itest:
	go test -p 1 ./integration_tests/...

.PHONY: unit-test
unit-test:
	go test -count=1 ./cmd/... ./pkg/...

.PHONY: apply
apply:
	helm install ambassador-agent ./helm/ambassador-agent -n ambassador --set image.pullPolicy=Always

.PHONY: delete
delete:
	helm delete ambassador-agent -n ambassador