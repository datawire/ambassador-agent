A8R_AGENT_VERSION ?= dev-latest
DEV_REGISTRY ?= datawiredev
IMAGE = ${DEV_REGISTRY}/ambassador-agent:${A8R_AGENT_VERSION}

.PHONY: build
build:
	CGO_ENABLED=0 go build \
	-trimpath \
	-ldflags=-X=main.version=${A8R_AGENT_VERSION} \
	-o=ambassador-agent \
	./cmd/main.go

.PHONY: generate
generate:
	mkdir -p ./pkg/api
	protoc \
    	-I=./api \
    	--proto_path=./api \
    	--go_opt=paths=source_relative \
    	--go_out=./pkg/api \
    	--go-grpc_opt=paths=source_relative \
    	--go-grpc_out=./pkg/api \
    	$(shell find ./api -iname "*.proto") 2>&1 > /dev/null

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
