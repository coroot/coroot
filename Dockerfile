FROM golang:1.23-bullseye AS backend-builder
RUN apt update && apt install -y liblz4-dev
WORKDIR /tmp/src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
ARG VERSION=unknown
RUN go build -mod=readonly -ldflags "-X main.version=$VERSION" -o coroot .


FROM registry.access.redhat.com/ubi9/ubi

ARG VERSION=unknown
LABEL name="coroot" \
      vendor="Coroot, Inc." \
      maintainer="Coroot, Inc." \
      version=${VERSION} \
      release="1" \
      summary="Coroot Community Edition." \
      description="Coroot Community Edition container image."

COPY LICENSE /licenses/LICENSE

COPY --from=backend-builder /tmp/src/coroot /opt/coroot/coroot

VOLUME /data
EXPOSE 8080

ENTRYPOINT ["/opt/coroot/coroot"]
