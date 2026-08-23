import path from "path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The Go server injects CLICKY_EXAMPLE_API_URL when launched via
// `entity-demo serve-ui --dev`, so Vite's proxy targets the same process.
const apiTarget = process.env.CLICKY_EXAMPLE_API_URL || "http://localhost:8080";

const clickyUiDist = path.resolve(
  __dirname,
  "../../../../clicky-ui/packages/ui/dist",
);

// `vite dev` (and `serve-ui --dev`) resolves clicky-ui from the sibling
// checkout so local library edits show up on reload. A production `vite build`
// must NOT: it would bundle whatever that checkout happens to hold while tsc
// type-checked the installed package, shipping a different version than was
// verified. Builds resolve @flanksource/clicky-ui from node_modules instead.
const devClickyUiAlias = {
  "@flanksource/clicky-ui/styles.css": path.join(clickyUiDist, "styles.css"),
  "@flanksource/clicky-ui/chat": path.join(clickyUiDist, "chat.js"),
  "@flanksource/clicky-ui/ai": path.join(clickyUiDist, "ai.js"),
  "@flanksource/clicky-ui/icons": path.join(clickyUiDist, "icons.js"),
  "@flanksource/clicky-ui": path.join(clickyUiDist, "index.js"),
};

export default defineConfig(({ command }) => ({
  // The example app is embedded and served from "/", so root-relative assets
  // keep deep links like /entity/:domainKey/:id working on full page loads.
  base: "/",
  plugins: [tailwindcss(), react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      ...(command === "serve" ? devClickyUiAlias : {}),
    },
    dedupe: ["react", "react-dom", "@tanstack/react-query"],
  },
  // Don't pre-bundle the linked clicky-ui: it's aliased to a sibling checkout's
  // dist, so a freshly exported symbol would otherwise 404 from a stale Vite
  // prebundle. Excluding it makes a clicky-ui `pnpm build` show up on reload.
  optimizeDeps: {
    exclude: ["@flanksource/clicky-ui"],
  },
  server: {
    fs: {
      // Setting `allow` replaces Vite's defaults, so list the webapp root itself
      // (index.html, src/, node_modules) alongside the out-of-root clicky-ui
      // dist the alias points at — otherwise Vite 403s its own index.html.
      allow: [__dirname, clickyUiDist],
    },
    proxy: {
      "/api": apiTarget,
      "/health": apiTarget,
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
}));
