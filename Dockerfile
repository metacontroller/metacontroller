FROM golang:1.15.8 AS build

ARG TAG
ENV TAG=${TAG:-dev}

COPY . /go/src/metacontroller.io/
WORKDIR /go/src/metacontroller.io/
ENV CGO_ENABLED=0
RUN make install

FROM alpine:3.13.1@sha256:08d6ca16c60fe7490c03d10dc339d9fd8ea67c6466dea8d558526b1330a85930
COPY --from=build /go/bin/metacontroller.io /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
