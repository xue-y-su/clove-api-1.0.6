ARG BUN_IMAGE=oven/bun:1@sha256:0733e50325078969732ebe3b15ce4c4be5082f18c4ac1a0f0ca4839c2e4e42a7
ARG GO_IMAGE=golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83
ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:f06537653ac770703bc45b4b113475bd402f451e85223f0f2837acbf89ab020a

FROM ${BUN_IMAGE} AS builder

WORKDIR /build/web
COPY web/package.json web/bun.lock ./
COPY web/default/package.json ./default/package.json
COPY web/classic/package.json ./classic/package.json
RUN bun install --frozen-lockfile
COPY ./web/shared ./shared
COPY ./web/default ./default
COPY ./VERSION /build/VERSION
RUN DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(tr -d '\r\n' < /build/VERSION) bun run --cwd default build

FROM ${BUN_IMAGE} AS builder-classic

WORKDIR /build/web
COPY web/package.json web/bun.lock ./
COPY web/default/package.json ./default/package.json
COPY web/classic/package.json ./classic/package.json
RUN bun install --frozen-lockfile
COPY ./web/shared ./shared
COPY ./web/classic ./classic
COPY ./VERSION /build/VERSION
RUN VITE_REACT_APP_VERSION=$(tr -d '\r\n' < /build/VERSION) bun run --cwd classic build

FROM ${GO_IMAGE} AS builder2
ENV GO111MODULE=on CGO_ENABLED=0

ARG GOPROXY=https://proxy.golang.org,direct
ARG TARGETOS
ARG TARGETARCH
ENV GOPROXY=${GOPROXY}
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /build/web/default/dist ./web/default/dist
COPY --from=builder-classic /build/web/classic/dist ./web/classic/dist
RUN go build -trimpath -buildvcs=false -ldflags "-s -w" -o healthcheck ./cmd/healthcheck
RUN go build -trimpath -buildvcs=false -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(tr -d '\r\n' < VERSION)'" -o new-api

FROM ${RUNTIME_IMAGE} AS runtime-files
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && groupadd --gid 10001 newapi \
    && useradd --uid 10001 --gid newapi --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin newapi \
    && install -d -o newapi -g newapi -m 0700 /data /app/logs /tmp \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

FROM scratch

COPY --from=runtime-files /etc/passwd /etc/passwd
COPY --from=runtime-files /etc/group /etc/group
COPY --from=runtime-files /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=runtime-files /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=runtime-files --chown=10001:10001 /data /data
COPY --from=runtime-files --chown=10001:10001 /app/logs /app/logs
COPY --from=runtime-files --chown=10001:10001 /tmp /tmp
COPY --from=builder2 /build/new-api /
COPY --from=builder2 /build/healthcheck /healthcheck
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/
EXPOSE 3000
WORKDIR /data
USER 10001:10001
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/healthcheck"]
ENTRYPOINT ["/new-api"]
