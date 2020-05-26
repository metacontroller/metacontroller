# This is the same as Dockerfile, but skips `dep ensure`.
# It assumes you already ran that locally.
FROM golang:1.10 AS build

COPY . /go/src/metacontroller.app/
WORKDIR /go/src/metacontroller.app/
RUN go install

FROM debian:stretch-slim
RUN apt-get update && apt-get install --no-install-recommends -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /go/bin/metacontroller.app /usr/bin/metacontroller
CMD ["/usr/bin/metacontroller"]
