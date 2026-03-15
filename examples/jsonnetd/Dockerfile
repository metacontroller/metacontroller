FROM golang:1.24.5 AS build

COPY . /go/src/jsonnetd/
WORKDIR /go/src/jsonnetd/
RUN go mod vendor && go install

FROM debian:trixie-slim
COPY --from=build /go/bin/jsonnetd /jsonnetd/
WORKDIR /jsonnetd
ENTRYPOINT ["/jsonnetd/jsonnetd"]
EXPOSE 8080