# Fiber middleware baseline (perf + security out of the box)

Wire these in order on every app. CORS is explicit origins, never `*`.

```go
app.Use(recover.New())
app.Use(requestid.New())
app.Use(logger.New())                              // early: a request rejected by
                                                   // a later middleware (429, CORS)
                                                   // still gets logged
app.Use(helmet.New())                              // security headers
app.Use(cors.New(cors.Config{AllowOrigins: origins})) // explicit, not "*"
app.Use(compress.New())
app.Use(etag.New())
app.Use(limiter.New(limiter.Config{               // explicit budget — the zero
    Max:        120,                               // value (~5/min) breaks real
    Expiration: 1 * time.Minute,                   // pages (each pulls many assets)
}))
```

The limiter keys on `c.IP()` by default. Behind a reverse proxy every request
arrives from the proxy's IP, so **all users share one bucket** unless you make
Fiber trust the forwarded client IP. Set that on the app config, not the
middleware:

```go
app := fiber.New(fiber.Config{
    ProxyHeader:             fiber.HeaderXForwardedFor,
    EnableTrustedProxyCheck: true,
    TrustedProxies:          []string{"10.0.0.0/8"}, // your proxy's CIDR
})
```

Only enable this when you actually sit behind a trusted proxy. Trusting
`X-Forwarded-For` from untrusted clients lets them spoof their IP and evade the
limiter.

**Do not put `cache.New()` in the global chain.** Fiber's cache keys on request
path by default, with no user/session in the key, so on an app with auth + SSR it
serves one user's rendered `GET /dashboard` to the next user for the whole TTL: a
cross-user data leak. Response-cache only *explicitly public, non-personalized* routes,
and only with a key that includes everything that varies the response:

```go
public := app.Group("/assets") // or a public, auth-free route group
public.Use(cache.New(cache.Config{
    Expiration:   10 * time.Minute,
    CacheControl: true,
    // KeyGenerator MUST include anything that changes the body (path is not enough
    // once cookies/headers/query vary the response). Never mount this on authed routes.
}))
```
For authenticated pages, rely on `etag` + per-handler `Cache-Control` instead of a
shared response cache.
