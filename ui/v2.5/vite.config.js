import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import legacy from "@vitejs/plugin-legacy";
import tsconfigPaths from "vite-tsconfig-paths";
import viteCompression from "vite-plugin-compression";
import { VitePWA } from "vite-plugin-pwa";

const nolegacy = process.env.VITE_APP_NOLEGACY === "true";
const sourcemap = process.env.VITE_APP_SOURCEMAPS === "true";

// https://vitejs.dev/config/
export default defineConfig(() => {
  let plugins = [
    react({
      babel: {
        compact: true,
      },
    }),
    tsconfigPaths(),
    VitePWA({
      strategies: "injectManifest",
      srcDir: "src",
      filename: "sw.ts",
      registerType: "prompt",
      injectRegister: false,
      manifest: false,
      injectManifest: {
        globPatterns: ["**/*.{js,css,html,svg,png,woff2}"],
        globIgnores: ["**/node_modules/**", "**/*-legacy-*"],
        maximumFileSizeToCacheInBytes: 5 * 1024 * 1024, // 5MB
      },
    }),
    viteCompression({
      algorithm: "gzip",
      deleteOriginFile: true,
      threshold: 0,
      filter: (file) => /\.(js|json|css|svg|md)$/i.test(file) && !file.endsWith("sw.js"),
    }),
  ];

  if (!nolegacy) {
    plugins = [...plugins, legacy()];
  }

  // Dev-only reverse proxy. The Vite dev server (:3000) forwards API, media
  // and websocket traffic to the running backend so the browser only ever
  // talks to :3000 (same-origin). This is what lets the session cookie from
  // multi-user auth flow through — a cross-origin :3000→:9999 fetch would
  // drop it and bounce you to the login page. Override the target with the
  // VITE_DEV_PROXY env var (default: the local Docker backend on :9999).
  // `server` is ignored by `vite build`, so production output is unaffected.
  const backend = process.env.VITE_DEV_PROXY || "http://localhost:9999";
  // changeOrigin:false — forward the browser's original Host header to the
  // backend. Stash bakes the request Host into every media URL it returns
  // (server.go BaseURLMiddleware: baseURL = scheme://r.Host). With
  // changeOrigin:true the Host would be rewritten to the target
  // (localhost:9999) and thumbnails/streams would come back as
  // http://localhost:9999/... — unreachable from a remote/LAN browser.
  // Keeping the real host (e.g. 192.168.1.171:5173) makes those URLs route
  // back through this proxy.
  const be = { target: backend, changeOrigin: false };
  const devProxy = {
    "^/graphql": { ...be, ws: true },
    "^/playground": be,
    "^/login": be,
    "^/logout": be,
    "^/css$": be,
    "^/javascript$": be,
    "^/customlocales": be,
    // Backend media/asset routes are SINGULAR; the SPA's client routes are
    // PLURAL (/scenes, /images, …). Anchor on a path boundary so e.g.
    // `/scenes` is served by Vite while `/scene/123/stream` is proxied.
    "^/scene(/|$)": be,
    "^/image(/|$)": be,
    "^/performer(/|$)": be,
    "^/studio(/|$)": be,
    "^/group(/|$)": be,
    "^/tag(/|$)": be,
    "^/gallery(/|$)": be,
    "^/downloads(/|$)": be,
    "^/plugin(/|$)": be,
    "^/share(/|$)": be,
    "^/custom(/|$)": be,
  };

  return {
    base: "",
    build: {
      outDir: "build",
      sourcemap: sourcemap,
      reportCompressedSize: false,
    },
    optimizeDeps: {
      entries: "src/index.tsx",
    },
    server: {
      // :5173 (Vite's canonical port), not :3000 — this host's :30xx range
      // is occupied by other services. strictPort makes a conflict fail
      // loudly instead of silently drifting to another port (which would
      // break the same-origin proxy assumption in .env.development.local).
      port: 5173,
      strictPort: true,
      cors: false,
      proxy: devProxy,
      watch: {
        // A stray in-project pnpm store (~107k files) blows past the system
        // inotify watch limit and crashes the dev server with ENOSPC. It's
        // never imported, so there's no reason to watch it.
        ignored: ["**/.pnpm-store/**"],
      },
    },
    publicDir: "public",
    assetsInclude: ["**/*.md"],
    plugins,
  };
});
