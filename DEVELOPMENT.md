# Fast Dev Loop (avoid the full Docker rebuild)

For day-to-day iteration you do **not** need `make docker-build` after every edit.
The full rebuild is only for baking a finished change into the deployed image.

## Frontend / theme / CSS / React — instant (no rebuild)

The Vite dev server gives hot reload against your **real** running instance and data.

```bash
make ui-start          # Vite dev server on http://localhost:5173
```

Then open **http://localhost:5173** — or from another device on the LAN,
**http://<this-host-ip>:5173** (e.g. `http://192.168.1.171:5173`). Log in there once;
the cookie is set for whatever origin you loaded.

- Edit anything under `ui/v2.5/src/**` (React, SCSS, the vendored theme CSS in
  `src/theme/css/*`) → the browser hot-reloads in <1s.
- Edit `ui/v2.5/public/theme/refract.js` (or any `public/` file) → it's served live;
  just refresh the browser (public files aren't HMR'd, but there's no rebuild).
- The PWA service worker is **not** active in dev, so there's no aggressive cache to
  fight — unlike the deployed `:9999` image.

### How it works (so you can debug it)

- In dev the frontend targets **the page's own host** on the dev port
  (`VITE_APP_PLATFORM_PORT=5173` in `ui/v2.5/.env.development.local`, gitignored) and
  Vite **reverse-proxies** API, media and websocket traffic to the backend — see
  `server.proxy` in `ui/v2.5/vite.config.js`. Keeping the host dynamic (not a
  hardcoded `localhost`) is what makes LAN access work; keeping it same-origin is
  what lets the multi-user auth cookie flow (a cross-origin `:5173→:9999` fetch would
  drop it). Don't hardcode `VITE_APP_PLATFORM_URL=http://localhost:5173` — that breaks
  access from any other device.
- Backend media routes are **singular** (`/scene`, `/image`, `/performer`…); the SPA's
  client routes are **plural** (`/scenes`, `/images`…). The proxy matches on a path
  boundary so `/scenes` stays in the SPA while `/scene/1/stream` is proxied.
- Point at a different backend with `VITE_DEV_PROXY=http://host:port make ui-start`.
- Dev server is pinned to `:5173` (`strictPort`) because this host's `:30xx` range is
  taken by other services.

## Backend (Go) changes — still compiled, but faster than a clean build

Go changes must be compiled into the binary. `make docker-build` is still the path,
but Docker layer caching means a **backend-only** change reuses the cached frontend
stage and the Go build cache, so only Go recompiles:

```bash
make docker-build && (cd docker/production && docker compose up -d --force-recreate stash)
```

The dev server on `:5173` proxies to this container, so once it restarts your dev UI
immediately sees the new backend — no dev-server restart needed.

## When to do the full rebuild

Do the full `make docker-build` + `--force-recreate` to **deploy** a finished change
into the `:9999` image (the "done" state), and for any backend change. Frontend
iteration lives entirely on `:5173`.
