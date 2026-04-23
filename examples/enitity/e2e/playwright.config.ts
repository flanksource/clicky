import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig, devices } from "@playwright/test";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const exampleDir = path.resolve(__dirname, "..");

const PORT = process.env.E2E_PORT ?? "18080";
const HOST = process.env.E2E_HOST ?? "127.0.0.1";
const BASE_URL = process.env.E2E_BASE_URL ?? `http://${HOST}:${PORT}`;
const GOCACHE = process.env.GOCACHE ?? "/tmp/go-build";

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  timeout: 45_000,
  expect: {
    timeout: 10_000,
  },
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
      },
    },
  ],
  webServer: {
    command: `cd "${exampleDir}" && GOCACHE="${GOCACHE}" task build && GOCACHE="${GOCACHE}" ./entity-demo serve-ui --host ${HOST} --port ${PORT}`,
    url: BASE_URL,
    reuseExistingServer: false,
    timeout: 240_000,
    stdout: "pipe",
    stderr: "pipe",
  },
});
