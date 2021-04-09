FROM golang:1.16.3 AS build

ARG TAG
ENV TAG=${TAG:-dev}

COPY . /go/src/metacontroller.io/
WORKDIR /go/src/metacontroller.io/
ENV CGO_ENABLED=0
RUN make install

FROM alpine:3.13.4@sha256:ec14c7992a97fc11425907e908340c6c3d6ff602f5f13d899e6b7027c9b4133a
COPY --from=build /go/bin/metacontroller.io /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
