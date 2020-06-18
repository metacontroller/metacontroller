FROM golang:1.10 AS build

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

COPY . /go/src/metacontroller.app/
WORKDIR /go/src/metacontroller.app/
ENV CGO_ENABLED=0
RUN dep ensure && go install

FROM alpine:3.12.0
COPY --from=build /go/bin/metacontroller.app /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
