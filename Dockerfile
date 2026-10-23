FROM alpine:3.24.3@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/metacontroller /usr/bin/metacontroller
RUN apk update && apk add --no-cache ca-certificates

# Run container as nonroot, use the same uid and naming convention as distroless images
# See https://github.com/GoogleContainerTools/distroless/blob/0d757ece34cdc83a2148cea6c697e262c333cb84/base/base.bzl#L8
RUN addgroup -g 65532 -S nonroot && adduser -D -u 65532 -g nonroot -S nonroot -G nonroot
USER nonroot:nonroot

CMD ["/usr/bin/metacontroller"]
