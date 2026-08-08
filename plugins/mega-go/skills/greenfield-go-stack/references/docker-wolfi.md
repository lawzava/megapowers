# Docker / Wolfi

Image tags and package availability change. Recheck the image publisher's
documentation before adopting this recipe.

Build a static binary, then choose the runner that matches its runtime needs.
Use `wolfi-base` when the final image needs `apk`; use `static` only when the
binary and copied runtime files need no package manager. Define health checks in
the deployment platform for an application's actual readiness endpoint.

```dockerfile
FROM golang:1-bookworm AS build
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/server

# Runner with apk available.
FROM cgr.dev/chainguard/wolfi-base AS wolfi
RUN apk add --no-cache ca-certificates
COPY --from=build /app /app
USER nonroot
ENTRYPOINT ["/app"]
```

Replace the `wolfi` final stage above with this final stage when using `static`.
It reuses the preceding `build` stage.

```dockerfile
# Static runner: copy any needed runtime files from a separate stage.
FROM cgr.dev/chainguard/wolfi-base AS certificates
RUN apk add --no-cache ca-certificates

FROM cgr.dev/chainguard/static
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /app /app
USER nonroot
ENTRYPOINT ["/app"]
```
