FROM golang:1.11 AS build

COPY . /go/src/thing-controller
WORKDIR /go/src/thing-controller
RUN go mod vendor && go build -o /go/bin/thing-controller

FROM debian:stretch-slim

COPY --from=build /go/bin/thing-controller /usr/bin/thing-controller
