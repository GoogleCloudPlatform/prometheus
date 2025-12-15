ARG IMAGE_BUILD_NODEJS=launcher.gcr.io/google/nodejs
ARG IMAGE_BUILD_GO=google-go.pkg.dev/golang:1.24.11@sha256:cafcfecdfc9ae0ab09e84613542f3a4da3f0b0eb4f61509f0d3614470a0e4b3c

ARG IMAGE_BASE_DEBUG=gcr.io/distroless/base-nossl-debian12:debug
ARG IMAGE_BASE=gke.gcr.io/gke-distroless/libc:gke_distroless_20251207.00_p0@sha256:43d5106aa4a3b743fde131c0446be322f398fa2964c819df19aafe4a475ae872

FROM ${IMAGE_BUILD_GO} AS gobase

# Compile the UI assets.
FROM ${IMAGE_BUILD_NODEJS} as assets
# To build the UI we need a recent node version and the go toolchain.
RUN install_node v17.9.0
COPY --from=gobase /usr/local/go /usr/local/
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . ./
RUN pwd
# Only build the UI but don't run ui-install as we vendor node_modules.
RUN make ui-build
RUN scripts/compress_assets.sh
RUN make npm_licenses

# Build the actual Go binary.
FROM gobase as buildbase
WORKDIR /app
COPY --from=assets /app ./
ENV GOEXPERIMENT=boringcrypto
ENV CGO_ENABLED=1
ENV GOFIPS140=off
ENV GOTOOLCHAIN=local
ENV GOARCH=${TARGETARCH}
ENV GOOS=${TARGETOS}
RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
      apt install -y --no-install-recommends \
        gcc-aarch64-linux-gnu libc6-dev-arm64-cross; \
      CC=aarch64-linux-gnu-gcc; \
    fi && \ 
    go build \
    -tags builtinassets -mod=vendor \
    -ldflags="-X github.com/prometheus/common/version.Version=$(cat VERSION) \
    -X github.com/prometheus/common/version.BuildDate=$(date --iso-8601=seconds)" \
    ./cmd/prometheus

# Configure distroless base image like the upstream Prometheus image.
# Since the directory and symlink setup needs shell access, we need yet another
# intermediate stage.
FROM ${IMAGE_BASE_DEBUG} as appbase

COPY documentation/examples/prometheus.yml  /etc/prometheus/prometheus.yml
COPY console_libraries/                     /usr/share/prometheus/console_libraries/
COPY consoles/                              /usr/share/prometheus/consoles/
RUN ["/busybox/sh", "-c", "ln -s /usr/share/prometheus/console_libraries /usr/share/prometheus/consoles/ /etc/prometheus/"]
RUN ["/busybox/sh", "-c", "mkdir -p /prometheus"]

FROM ${IMAGE_BASE}

COPY --from=buildbase /app/prometheus /bin/prometheus
COPY --from=appbase --chown=nobody:nobody /etc/prometheus /etc/prometheus
COPY --from=appbase --chown=nobody:nobody /prometheus /prometheus
COPY --from=appbase /usr/share/prometheus /usr/share/prometheus
COPY LICENSE /LICENSE
COPY NOTICE /NOTICE
COPY --from=assets /app/npm_licenses.tar.bz2 /npm_licenses.tar.bz2

USER       nobody
EXPOSE     9090
VOLUME     [ "/prometheus" ]
ENTRYPOINT [ "/bin/prometheus" ]
CMD        [ "--config.file=/etc/prometheus/prometheus.yml", \
             "--storage.tsdb.path=/prometheus", \
             "--web.console.libraries=/usr/share/prometheus/console_libraries", \
             "--web.console.templates=/usr/share/prometheus/consoles" ]
