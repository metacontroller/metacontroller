FROM golang:1.14.4@sha256:d2b31e71040ae90fc0e115a73c7657f6e48bc4c2642a48f4b68406761868b914 AS build

COPY . /go/src/metacontroller.io/
WORKDIR /go/src/metacontroller.io/
ENV CGO_ENABLED=0
RUN make vendor && go install

FROM alpine:3.12.2@sha256:25f5332d060da2c7ea2c8a85d2eac623bd0b5f97d508b165f846c7d172897438
COPY --from=build /go/bin/metacontroller.io /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates openssl=1.1.1i-r0 libcrypto1.1=1.1.1i-r0 libssl1.1=1.1.1i-r0
CMD ["/usr/bin/metacontroller"]
