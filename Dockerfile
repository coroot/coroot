FROM golang:1.18-buster AS backend-builder
RUN apt update && apt install -y liblz4-dev
WORKDIR /tmp/src
COPY go.mod go.sum .
RUN go mod download
COPY . .
ARG VERSION=unknown
RUN go test ./...
RUN go install -mod=readonly -ldflags "-X main.version=$VERSION" .


FROM node:18-buster AS frontend-builder
WORKDIR /tmp/src
COPY ./front/package*.json ./
RUN npm install
COPY ./front .
RUN npx vue-cli-service build --dest=static src/main.js


FROM alpine:3.19
RUN apk add --no-cache ca-certificates

WORKDIR /opt/coroot
COPY --from=backend-builder /go/bin/coroot /opt/coroot/coroot
COPY --from=frontend-builder /tmp/src/static /opt/coroot/static

VOLUME /data
EXPOSE 8080

ENTRYPOINT ["/opt/coroot/coroot"]
