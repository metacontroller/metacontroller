FROM alpine:3.16.1@sha256:7580ece7963bfa863801466c0a488f11c86f85d9988051a9f9c68cb27f6b7872
COPY metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates

# Run container as nonroot, use the same uid and naming convention as distroless images
# See https://github.com/GoogleContainerTools/distroless/blob/0d757ece34cdc83a2148cea6c697e262c333cb84/base/base.bzl#L8
RUN addgroup -g 65532 -S nonroot && adduser -D -u 65532 -g nonroot -S nonroot -G nonroot
USER nonroot:nonroot

CMD ["/usr/bin/metacontroller"]
