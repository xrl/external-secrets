# syntax=docker/dockerfile:1
# The compiler runs on the native CI host. This shell stage is only the
# preparation seam for the later SDK integration, never a cross-build stage.
FROM golang:1.26.5-bookworm AS prepare
ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG TARGETARCH
COPY bin/external-secrets-linux-${TARGETARCH} /bin/external-secrets
COPY hack/xrl-image-prepare.sh /prepare.sh
RUN test "$BUILDPLATFORM" = "$TARGETPLATFORM"
RUN --network=none sh /prepare.sh /bin/external-secrets /image-root

FROM gcr.io/distroless/static@sha256:9197324ba51d9cd071af8505989365c006adf9d6d2067eada25aef00abbb5278
ARG REVISION
ARG VERSION
LABEL org.opencontainers.image.source="https://github.com/xrl/external-secrets" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.version="${VERSION}"
COPY --from=prepare /bin/external-secrets /bin/external-secrets
COPY --from=prepare /image-root/ /
USER 65534
ENTRYPOINT ["/bin/external-secrets"]
