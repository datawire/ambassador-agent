FROM golang:alpine3.17 as build-stage
RUN apk update && \
    apk add --no-cache gcc musl-dev bash make protoc protobuf-dev && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
WORKDIR /build
COPY . .
ARG A8R_AGENT_VERSION

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    make build

FROM alpine:3.15
COPY --from=build-stage /build/build-output/bin/ambassador-agent /usr/local/bin

EXPOSE 8080

CMD [ "ambassador-agent" ]
