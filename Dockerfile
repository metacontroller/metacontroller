FROM golang:1.10 AS build

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

COPY . /go/src/k8s.io/metacontroller/
WORKDIR /go/src/k8s.io/metacontroller/
RUN dep ensure && go install

FROM debian:stretch-slim
COPY --from=build /go/bin/metacontroller /usr/bin/
CMD ["/usr/bin/metacontroller"]