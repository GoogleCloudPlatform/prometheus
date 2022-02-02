ARG IMAGE_BUILD_NODEJS=launcher.gcr.io/google/nodejs
ARG IMAGE_BUILD_GO=golang:1.17-buster
ARG IMAGE_BASE_DEBUG=gcr.io/distroless/static-debian10:debug
ARG IMAGE_BASE=gcr.io/distroless/static-debian10

FROM ${IMAGE_BUILD_GO} AS gobase

# Compile the UI assets.
FROM ${IMAGE_BUILD_NODEJS} as assets
# To build the UI we need a recent node version and the go toolchain.
RUN install_node v14.16.1
COPY --from=gobase /usr/local/go /usr/local/
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /app
COPY . ./
RUN make assets
RUN make npm_licenses

# Build the actual Go binary.
FROM gobase as buildbase
WORKDIR /app
COPY --from=assets /app ./
RUN CGO_ENABLED=0 go build \
    -tags builtinassets -mod=vendor \
    -ldflags="-X github.com/prometheus/common/version.Version=$(cat VERSION) \
    -X github.com/prometheus/common/version.BuildDate=$(date --iso-8601=seconds)" \
    ./cmd/prometheus
RUN CGO_ENABLED=0 go build \
    -mod=vendor \
    -ldflags="-X github.com/prometheus/common/version.Version=$(cat VERSION) \
    -X github.com/prometheus/common/version.BuildDate=$(date --iso-8601=seconds)" \
    ./cmd/promtool

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
COPY --from=buildbase /app/promtool /bin/promtool
COPY --from=appbase --chown=nobody:nobody /etc/prometheus /etc/prometheus
COPY --from=appbase --chown=nobody:nobody /prometheus /prometheus
COPY --from=appbase /usr/share/prometheus /usr/share/prometheus
COPY LICENSE /LICENSE
COPY NOTICE /NOTICE
COPY --from=assets /app/npm_licenses.tar.bz2 /npm_licenses.tar.bz2

USER       nobody
EXPOSE     9090
VOLUME     [ "/prometheus" ]
WORKDIR    /prometheus
ENTRYPOINT [ "/bin/prometheus" ]
CMD        [ "--config.file=/etc/prometheus/prometheus.yml", \
             "--storage.tsdb.path=/prometheus", \
             "--web.console.libraries=/usr/share/prometheus/console_libraries", \
             "--web.console.templates=/usr/share/prometheus/consoles" ]
