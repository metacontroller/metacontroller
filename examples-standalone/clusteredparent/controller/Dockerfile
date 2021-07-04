FROM golang:1.16.5 AS build

COPY . /code
WORKDIR /code/examples-standalone/clusteredparent/controller

ENV CGO_ENABLED=0
RUN go mod download
RUN go build -o /usr/bin/metacontroller main.go

FROM alpine:3.14.0@sha256:234cb88d3020898631af0ccbbcca9a66ae7306ecd30c9720690858c1b007d2a0
COPY --from=build /usr/bin/metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
