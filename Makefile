A8R_AGENT_VERSION ?= dev-latest
DEV_REGISTRY ?= datawiredev
IMAGE = ${DEV_REGISTRY}/ambassador-agent:${A8R_AGENT_VERSION}
BUILDDIR=build-output

include build-aux/tools.mk

$(BUILDDIR)/go1%.src.tar.gz:
	mkdir -p $(BUILDDIR)
	curl -o $@ --fail -L https://dl.google.com/go/$(@F)

.PHONY: build
build:
	CGO_ENABLED=0 go build \
	-trimpath \
	-ldflags=-X=main.version=${A8R_AGENT_VERSION} \
	-o=ambassador-agent \
	./cmd/main.go

.PHONY: format
format: $(tools/golangci-lint) ## (QA) Automatically fix linter complaints
	$(tools/golangci-lint) run --fix --timeout 2m ./... || true

lint: $(tools/golangci-lint) ## (QA) Run the linter
	$(tools/golangci-lint) run --timeout 8m ./...

.PHONY: generate
generate: ## (Generate) Update generated files that get checked in to Git
generate: generate-clean
generate: $(tools/protoc) $(tools/protoc-gen-go) $(tools/protoc-gen-go-grpc) $(tools/go-mkopensource) $(BUILDDIR)/$(shell go env GOVERSION).src.tar.gz
generate:
	mkdir -p ./pkg/api
	$(tools/protoc) \
    	-I=./api \
    	--proto_path=./api \
    	--go_opt=paths=source_relative \
    	--go_out=./pkg/api \
    	--go-grpc_opt=paths=source_relative \
    	--go-grpc_out=./pkg/api \
    	$(shell find ./api -iname "*.proto") 2>&1 > /dev/null


	mkdir -p $(BUILDDIR)
		$(tools/go-mkopensource) --gotar=$(filter %.src.tar.gz,$^) --output-format=txt --package=mod --application-type=external  \
			--unparsable-packages build-aux/unparsable-packages.yaml >$(BUILDDIR)/DEPENDENCIES.txt
		sed 's/\(^.*the Go language standard library ."std".[ ]*v[1-9]\.[1-9]*\)\..../\1    /' $(BUILDDIR)/DEPENDENCIES.txt >DEPENDENCIES.md

		printf "ambassador-agent incorporates Free and Open Source software under the following licenses:\n\n" > DEPENDENCY_LICENSES.md
		$(tools/go-mkopensource) --gotar=$(filter %.src.tar.gz,$^) --output-format=txt --package=mod \
			--output-type=json --application-type=external --unparsable-packages build-aux/unparsable-packages.yaml > $(BUILDDIR)/DEPENDENCIES.json
		jq -r '.licenseInfo | to_entries | .[] | "* [" + .key + "](" + .value + ")"' $(BUILDDIR)/DEPENDENCIES.json > $(BUILDDIR)/LICENSES.txt
		sed -e 's/\[\([^]]*\)]()/\1/' $(BUILDDIR)/LICENSES.txt >> DEPENDENCY_LICENSES.md

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
itest: image-push
	go test -count=1 ./integration_tests/...

.PHONY: unit-test
unit-test:
	go test -count=1 ./cmd/... ./pkg/...

.PHONY: apply
apply:
	helm install ambassador-agent ./helm/ambassador-agent -n ambassador --set image.pullPolicy=Always

.PHONY: delete
delete:
	helm delete ambassador-agent -n ambassador