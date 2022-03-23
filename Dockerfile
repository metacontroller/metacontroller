FROM golang:1.18.0 AS build

ARG TAG
ENV TAG=${TAG:-dev}

COPY . /go/src/metacontroller/
WORKDIR /go/src/metacontroller/
ENV CGO_ENABLED=0
RUN make install

FROM alpine:3.15.2@sha256:66b861b1099af1551a0eee163c175fd008744192c3fbb7f22e998db0ce09e8ea
COPY --from=build /go/bin/metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
