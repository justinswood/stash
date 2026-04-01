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
      port: 3000,
      cors: false,
    },
    publicDir: "public",
    assetsInclude: ["**/*.md"],
    plugins,
  };
});
