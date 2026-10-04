# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
COPY *.go ./
RUN go mod download && CGO_ENABLED=0 go build -ldflags="-s -w" -o /demo-api .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /demo-api /demo-api
EXPOSE 4000
ENTRYPOINT ["/demo-api"]
