FROM node:21 AS frontend-builder
WORKDIR /tmp/src
COPY . .
WORKDIR /tmp/src/front

RUN npm ci
RUN npm run build-prod


FROM golang:1.23-bullseye AS backend-builder
ARG VERSION=unknown

RUN apt update && apt install -y liblz4-dev
WORKDIR /tmp/src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
WORKDIR /tmp/src/static
COPY --from=frontend-builder /tmp/src/static /tmp/src/static
WORKDIR /tmp/src
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

COPY --from=backend-builder /tmp/src/coroot /usr/bin/coroot
RUN mkdir /data && chown 65534:65534 /data

USER 65534:65534
VOLUME /data
EXPOSE 8080

ENTRYPOINT ["/usr/bin/coroot"]
