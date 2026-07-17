FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go test ./... \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wikiforge ./cmd/wikiforge

FROM node:22-bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates chromium \
    && npm install -g openwiki@0.2.0 @mermaid-js/mermaid-cli@11.12.0 \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/*
ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium \
    OPENWIKI_TELEMETRY_DISABLED=1
COPY --from=build /out/wikiforge /usr/local/bin/wikiforge
WORKDIR /workspace
ENTRYPOINT ["wikiforge"]
