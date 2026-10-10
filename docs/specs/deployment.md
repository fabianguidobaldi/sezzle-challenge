# Deployment spec

The app runs as two containers managed by Docker Compose: nginx serves the SPA and proxies the API, and the Go backend stays a pure JSON API.

```
browser ──► frontend (nginx :8080) ──┬── /            → static SPA (dist/)
                                     └── /api/*, /healthz → backend (Go :8080, internal only)
```

```sh
docker compose up --build   # → http://localhost:8080
```

## 1. Files

```
├── compose.yaml
├── backend/Dockerfile
└── frontend/
    ├── Dockerfile
    └── nginx.conf
```

## 2. Backend image (`backend/Dockerfile`)

Multi-stage build:
1. `golang:1.26-alpine`: `go mod download`, then `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server ./cmd/server`.
2. `gcr.io/distroless/static-debian12:nonroot`: copies `/server` only. Runs as non-root and exposes port 8080.

## 3. Frontend image (`frontend/Dockerfile`)

Multi-stage build:
1. `node:24-alpine`: `npm ci`, then `npm run build` (output in `dist/`).
2. `nginxinc/nginx-unprivileged:alpine`: copies `dist/` and `nginx.conf`. Runs as non-root and listens on 8080.

### `nginx.conf`

- `location /api/` and `location = /healthz`: `proxy_pass http://backend:8080`, forwarding `Host`, `X-Forwarded-For` and `X-Forwarded-Proto`.
- `location /assets/`: Vite's hashed files, served with `Cache-Control: public, max-age=31536000, immutable`.
- `location /`: `try_files $uri /index.html` (SPA fallback); `index.html` gets `Cache-Control: no-cache` so new deploys are picked up straight away.
- `gzip on` for text, JS, CSS and JSON.
- Basic security headers: `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and a restrictive `Content-Security-Policy` (`default-src 'self'`).

## 4. `compose.yaml`

| Service    | Build        | Ports         | Notes |
| ---------- | ------------ | ------------- | ----- |
| `backend`  | `./backend`  | none published | Reachable only on the compose network as `backend:8080`. Healthcheck: `["/server", "-healthcheck"]`. |
| `frontend` | `./frontend` | `8080:8080`   | `depends_on: backend (condition: service_healthy)`, so nginx starts only once the API is up. |

Both services use `restart: unless-stopped`.

API calls go through the single public port, e.g. `curl localhost:8080/api/v1/calculate ...`. To call the backend directly during development, run it with `go run ./cmd/server`.

## 5. Why not a single image

- The backend stays a focused microservice with no static-file or SPA-fallback code.
- nginx does what it's good at: caching headers, compression and SPA fallback.
- The frontend and backend can be rebuilt, deployed and scaled independently. This mirrors a real setup, where the SPA sits on a CDN or static host and the API sits behind a proxy.
- Running both processes in one container would need a process supervisor, which is an anti-pattern.

The brief asks for "a Dockerfile". The README explains that this setup uses two Dockerfiles plus `compose.yaml`, and still runs with one command.
