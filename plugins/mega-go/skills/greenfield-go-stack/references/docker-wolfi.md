# Docker / Wolfi

Multi-stage: build on `golang` with `CGO_ENABLED=0`, run on
`cgr.dev/chainguard/wolfi-base` (or `static`). Non-root, healthcheck. Use
`docker compose` for local (app + sidecars).

```dockerfile
FROM golang:1-bookworm AS build
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/server

FROM cgr.dev/chainguard/wolfi-base
RUN apk add --no-cache ca-certificates
COPY --from=build /app /app
USER nonroot
ENTRYPOINT ["/app"]
```
