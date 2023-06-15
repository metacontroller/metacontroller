FROM alpine:3.18.2@sha256:82d1e9d7ed48a7523bdebc18cf6290bdb97b82302a8a9c27d4fe885949ea94d1
COPY metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates

# Run container as nonroot, use the same uid and naming convention as distroless images
# See https://github.com/GoogleContainerTools/distroless/blob/0d757ece34cdc83a2148cea6c697e262c333cb84/base/base.bzl#L8
RUN addgroup -g 65532 -S nonroot && adduser -D -u 65532 -g nonroot -S nonroot -G nonroot
USER nonroot:nonroot

CMD ["/usr/bin/metacontroller"]
