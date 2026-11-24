FROM golang:1.27.4-trixie AS builder

ENV CGO_ENABLED=0

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY mock.yml ./mock.yml

RUN go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/rfc-server ./cmd/rfc-server \
    && go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/rfc-call ./cmd/rfc-call \
    && sed 's/^listen_host:.*/listen_host: 0.0.0.0/' mock.yml > /out/mock.yml

FROM debian:trixie-slim AS runtime

RUN groupadd --gid 10001 rfc \
    && useradd --uid 10001 --gid rfc --no-create-home --shell /usr/sbin/nologin rfc \
    && install -d -o rfc -g rfc -m 0755 /app

COPY --from=builder /out/rfc-server /out/rfc-call /usr/local/bin/
COPY --from=builder --chown=rfc:rfc /out/mock.yml /app/mock.yml

WORKDIR /app
USER rfc
EXPOSE 3313/tcp
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/rfc-server"]
CMD ["-config", "/app/mock.yml"]
