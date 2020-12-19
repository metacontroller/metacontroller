FROM golang:1.15.6 AS build

COPY . /go/src/metacontroller.io/
WORKDIR /go/src/metacontroller.io/
ENV CGO_ENABLED=0
RUN make vendor && go install

FROM alpine:3.12.3@sha256:074d3636ebda6dd446d0d00304c4454f468237fdacf08fb0eeac90bdbfa1bac7
COPY --from=build /go/bin/metacontroller.io /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates
CMD ["/usr/bin/metacontroller"]
