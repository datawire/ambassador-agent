A8R_AGENT_VERSION ?= "dev-latest"

.PHONY: build
build: generate
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
	docker build --tag ambassador/ambassador-agent:${A8R_AGENT_VERSION} .

.PHONY: itest
itest:
	go test ./integration_tests/...
