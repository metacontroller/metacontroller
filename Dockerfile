FROM golang:1.16.6 AS build

ARG TAG
ENV TAG=${TAG:-dev}

COPY . /go/src/metacontroller/
WORKDIR /go/src/metacontroller/
ENV CGO_ENABLED=0
RUN make install

FROM alpine:3.14.0@sha256:adab3844f497ab9171f070d4cae4114b5aec565ac772e2f2579405b78be67c96
COPY --from=build /go/bin/metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
