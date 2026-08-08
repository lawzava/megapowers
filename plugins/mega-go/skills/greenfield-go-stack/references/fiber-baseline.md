# Fiber v3 middleware baseline

Verified against Fiber v3 documentation on 2026-08-08. Recheck
[current Fiber documentation](https://docs.gofiber.io/) before adopting this
recipe.

Use only when Fiber v3 is an intentional framework choice. Configure CORS for
the application's known browser origins.

```go
app.Use(recover.New())
app.Use(requestid.New())
app.Use(logger.New())                              // early: a request rejected by
                                                   // a later middleware (429, CORS)
                                                   // still gets logged
app.Use(helmet.New())                              // security headers
app.Use(cors.New(cors.Config{AllowOrigins: origins}))
app.Use(compress.New())
app.Use(etag.New())
app.Use(limiter.New(limiter.Config{               // explicit budget — the zero
    Max:        120,                               // value (~5/min) breaks real
    Expiration: 1 * time.Minute,                   // pages (each pulls many assets)
}))
```

The limiter keys on `c.IP()` by default. Behind a reverse proxy, configure only
proxies you control before reading forwarded client IPs:

```go
app := fiber.New(fiber.Config{
    TrustProxy: true,
    TrustProxyConfig: fiber.TrustProxyConfig{
        Proxies: []string{"10.0.0.0/8"}, // your proxy's CIDR
    },
    ProxyHeader: fiber.HeaderXForwardedFor,
})
```

Without trusted-proxy configuration, `c.IP()` uses the remote TCP IP. Trusting
uncontrolled `X-Forwarded-For` lets clients spoof their IP and evade the
limiter.

**Do not put `cache.New()` in the global chain.** Fiber v3's default key uses
the HTTP method, path, canonical query, and selected representation headers. It
does not include cookies, so response-cache only *explicitly public,
non-personalized* routes. Add cookie keying only when the route is designed for
it, never as a way to cache authenticated pages:

```go
public := app.Group("/assets") // or a public, auth-free route group
public.Use(cache.New(cache.Config{
    Expiration: 10 * time.Minute,
    // v3 emits Cache-Control by default. Set DisableCacheControl only when
    // another layer owns that header.
}))
```
For authenticated pages, rely on `etag` + per-handler `Cache-Control` instead of a
shared response cache.
