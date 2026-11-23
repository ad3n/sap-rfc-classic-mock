FROM golang:1.27.4-trixie AS builder

ENV CGO_ENABLED=1 \
    SAPNWRFC_HOME=/usr/local/sap/nwrfcsdk \
    CGO_CFLAGS="-std=gnu17 -I/usr/local/sap/nwrfcsdk/include" \
    CGO_LDFLAGS="-L/usr/local/sap/nwrfcsdk/lib -Wl,-rpath,/usr/local/sap/nwrfcsdk/lib"

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY docker/nwrfcsdk /usr/local/sap/nwrfcsdk
COPY cmd ./cmd
COPY internal ./internal
COPY mock.yml ./mock.yml

RUN go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/rfc-server ./cmd/rfc-server \
    && go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/rfc-call ./cmd/rfc-call \
    && sed 's/^listen_host:.*/listen_host: 0.0.0.0/' mock.yml > /out/mock.yml

FROM debian:trixie-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libgcc-s1 libstdc++6 libuuid1 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 rfc \
    && useradd --uid 10001 --gid rfc --no-create-home --home-dir /app rfc \
    && install -d -o rfc -g rfc /app

COPY --from=builder /usr/local/sap/nwrfcsdk/lib /usr/local/sap/nwrfcsdk/lib
RUN echo '/usr/local/sap/nwrfcsdk/lib' > /etc/ld.so.conf.d/nwrfcsdk.conf \
    && ldconfig

COPY --from=builder /out/rfc-server /out/rfc-call /usr/local/bin/
COPY --from=builder --chown=rfc:rfc /out/mock.yml /app/mock.yml

WORKDIR /app
USER rfc
EXPOSE 3313/tcp
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/rfc-server"]
CMD ["-config", "/app/mock.yml"]
