import path from "path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The Go server injects CLICKY_EXAMPLE_API_URL when launched via
// `entity-demo serve-ui --dev`, so Vite's proxy targets the same process.
const apiTarget = process.env.CLICKY_EXAMPLE_API_URL || "http://localhost:8080";

export default defineConfig({
  // The example app is embedded and served from "/", so root-relative assets
  // keep deep links like /entity/:domainKey/:id working on full page loads.
  base: "/",
  plugins: [tailwindcss(), react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@flanksource/clicky-ui": path.resolve(
        __dirname,
        "../../../../clicky-ui/packages/ui/dist/index.mjs",
      ),
      "@flanksource/clicky-ui/styles.css": path.resolve(
        __dirname,
        "../../../../clicky-ui/packages/ui/src/styles/tokens.css",
      ),
    },
    dedupe: ["react", "react-dom", "@tanstack/react-query"],
  },
  server: {
    fs: {
      allow: [
        path.resolve(__dirname, "../../../../clicky-ui/packages/ui/dist"),
        path.resolve(__dirname, "../../../../clicky-ui/packages/ui/src/styles"),
      ],
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
});
