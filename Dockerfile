# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd AS build
ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0 GOFLAGS=-mod=readonly
WORKDIR /src
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download && go mod verify
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN mkdir -m 1777 /out/tmp

FROM scratch
LABEL org.opencontainers.image.title="document-module-microservice" \
      org.opencontainers.image.description="Go microservice converting between JSON, XML, CSV, TSV, TOML, YAML, MessagePack, XLSX and .properties and generating PDF, DOCX and XLSX documents" \
      org.opencontainers.image.source="https://github.com/SantiagoLopezDeharo/document-module-microservice" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /out/server /server
# Large XLSX worksheets are streamed through temporary files.
COPY --from=build /out/tmp /tmp
ENV SERVER_ADDRESS=:8080
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/server"]
