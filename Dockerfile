FROM golang:1.16.5 AS build

ARG TAG
ENV TAG=${TAG:-dev}

COPY . /go/src/metacontroller/
WORKDIR /go/src/metacontroller/
ENV CGO_ENABLED=0
RUN make install

FROM alpine:3.13.5@sha256:f51ff2d96627690d62fee79e6eecd9fa87429a38142b5df8a3bfbb26061df7fc
COPY --from=build /go/bin/metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
