FROM golang:1.16.4 AS build

ARG TAG
ENV TAG=${TAG:-dev}

COPY . /go/src/metacontroller/
WORKDIR /go/src/metacontroller/
ENV CGO_ENABLED=0
RUN make install

FROM alpine:3.13.5@sha256:69e70a79f2d41ab5d637de98c1e0b055206ba40a8145e7bddb55ccc04e13cf8f
COPY --from=build /go/bin/metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
