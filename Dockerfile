# diary-relay: Go API + Node/Hexo for preview generation + embedded web editor
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/diary-relay ./cmd/server

FROM node:20-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/diary-relay /usr/local/bin/diary-relay
COPY config.example.yaml /app/config.example.yaml

# Hexo CLI available via npx; blog_workdir must be mounted with node_modules + theme.
ENV NODE_PATH=/app
EXPOSE 8080
VOLUME ["/data", "/blog", "/preview"]
# Example:
#   docker run --rm -p 8080:8080 \
#     -v $PWD/config.yaml:/app/config.yaml:ro \
#     -v /path/to/blog:/blog \
#     -v preview:/preview -v data:/data \
#     diary-relay -config /app/config.yaml
ENTRYPOINT ["diary-relay"]
CMD ["-config", "/app/config.yaml"]
