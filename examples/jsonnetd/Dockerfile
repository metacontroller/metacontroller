FROM golang:1.10 AS build

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

COPY . /go/src/jsonnetd/
WORKDIR /go/src/jsonnetd/
RUN dep ensure && go install

FROM debian:stretch-slim
COPY --from=build /go/bin/jsonnetd /jsonnetd/
WORKDIR /jsonnetd
ENTRYPOINT ["/jsonnetd/jsonnetd"]
EXPOSE 8080