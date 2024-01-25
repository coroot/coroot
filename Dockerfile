FROM golang:1.21-bullseye AS backend-builder
RUN apt update && apt install -y liblz4-dev
WORKDIR /tmp/src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
ARG VERSION=unknown
RUN go build -mod=readonly -ldflags "-X main.version=$VERSION" -o coroot .


FROM node:21-bullseye AS frontend-builder
WORKDIR /tmp/src/front
COPY ./front/package*.json ./
RUN npm ci
COPY ./front .
RUN npm run prod


FROM debian:bullseye
RUN apt update && apt install -y ca-certificates && apt clean

WORKDIR /opt/coroot
COPY --from=backend-builder /tmp/src/coroot /opt/coroot/coroot
COPY --from=frontend-builder /tmp/src/static /opt/coroot/static

VOLUME /data
EXPOSE 8080

ENTRYPOINT ["/opt/coroot/coroot"]
