FROM golang:1.18-stretch AS backend-builder
WORKDIR /go/src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go install -mod=readonly .
RUN go test ./...


FROM node:18-buster AS frontend-builder
WORKDIR /tmp/front
COPY ./front/package*.json ./
RUN npm install
COPY ./front .
RUN ./node_modules/.bin/vue-cli-service build --dest=dist src/main.js


FROM debian:stretch

RUN apt update && apt install -y ca-certificates && apt clean

WORKDIR /opt/coroot

COPY --from=backend-builder /go/bin/coroot /opt/coroot/coroot
COPY --from=frontend-builder /tmp/front/dist /opt/coroot/static

VOLUME /data
EXPOSE 8080

ENTRYPOINT ["/opt/coroot/coroot", "--datadir=/data"]