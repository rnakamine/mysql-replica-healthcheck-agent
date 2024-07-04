FROM golang:1-bullseye AS builder
WORKDIR /workdir/
COPY . .
RUN apt-get update && \
  apt-get install -y --no-install-recommends make && \
  make build && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*
RUN make build

FROM debian:bullseye-slim
COPY --from=builder /workdir/mysql-replica-healthcheck-agent ./usr/bin

ENTRYPOINT ["/entrypoint.sh"]

COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
