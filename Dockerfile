FROM golang:1.10 AS build

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

COPY . /go/src/metacontroller.app/
WORKDIR /go/src/metacontroller.app/
RUN dep ensure && go install

FROM debian:stretch-slim
RUN apt-get update && apt-get install --no-install-recommends -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /go/bin/metacontroller.app /usr/bin/metacontroller
CMD ["/usr/bin/metacontroller"]
