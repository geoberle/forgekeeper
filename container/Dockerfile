FROM golang:1.25.5-alpine AS orchestrator-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /orchestrator ./cmd/orchestrator

FROM ghcr.io/fabro-sh/fabro@sha256:5440f204b70da0a4165b34d8591fb9461317bd98fa138ae61ec9095052d599c3
USER root
RUN apk add --no-cache nodejs npm bash python3 ripgrep github-cli go build-base \
    && python3 -m venv /opt/python \
    && /opt/python/bin/pip install --no-cache-dir PyYAML==6.0.3 \
    && npm install --global @agentclientprotocol/codex-acp@2.6.0 \
    && npm cache clean --force \
    && ln -s /usr/local/lib/node_modules/@agentclientprotocol/codex-acp/node_modules/@openai/codex/bin/codex.js /usr/local/bin/codex \
    && sed -i 's|:/var/fabro:|:/storage/home:|' /etc/passwd
# Petri checkout archives can contain GNU sparse entries unsupported by BusyBox tar.
RUN apk add --no-cache tar lz4
COPY --from=orchestrator-build /orchestrator /usr/local/bin/orchestrator
COPY bundle /opt/experiment/bundle
COPY scripts /experiment/scripts
COPY workflows /experiment/workflows
COPY fixtures /experiment/fixtures
RUN chmod -R a+rX /opt/experiment/bundle /experiment
ENV PATH="/opt/python/bin:$PATH" \
    HOME=/storage/home \
    CODEX_HOME=/storage/home/.codex \
    FABRO_NO_UPGRADE_CHECK=1
