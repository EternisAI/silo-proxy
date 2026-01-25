FROM golang:1.25 AS build-env

ARG APP=silo-proxy-server
ARG VERSION=dev

ENV GO111MODULE=on  \
    CGO_ENABLED=0   \
    GOOS=linux      \
    GOARCH=amd64

WORKDIR /build
COPY cmd/${APP}/ ${APP}/
COPY internal/ internal/
COPY proto/ proto/
COPY go.mod .
COPY go.sum .
RUN go build -o app -ldflags "-X main.AppVersion=${VERSION}" ${APP}/*.go 


FROM alpine

ARG APP=silo-proxy-server

WORKDIR /go/bin
COPY cmd/${APP}/application.yml /go/bin/application.yml
COPY --from=build-env /build/app .
RUN chmod +x app

ENTRYPOINT ["/go/bin/app"] 


