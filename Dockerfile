FROM alpine:3.16.3@sha256:b95359c2505145f16c6aa384f9cc74eeff78eb36d308ca4fd902eeeb0a0b161b
COPY metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates

# Run container as nonroot, use the same uid and naming convention as distroless images
# See https://github.com/GoogleContainerTools/distroless/blob/0d757ece34cdc83a2148cea6c697e262c333cb84/base/base.bzl#L8
RUN addgroup -g 65532 -S nonroot && adduser -D -u 65532 -g nonroot -S nonroot -G nonroot
USER nonroot:nonroot

CMD ["/usr/bin/metacontroller"]
