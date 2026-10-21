# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && mkdir -m 1777 /out/tmp

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
