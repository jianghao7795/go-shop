import { defineConfig } from "vite";
import wails from "@wailsio/runtime/plugins/vite";
import vue from "@vitejs/plugin-vue";

/** Fallback port used when WAILS_VITE_PORT is missing or invalid. */
const DEFAULT_PORT = 9245;
/** Loopback only: the Wails WebView loads the dev server from the same machine. */
const DEFAULT_HOST = "0.0.0.0";

/**
 * Minimal typed accessor for `process.env`, so this config file stays valid
 * without pulling in `@types/node` (which the project does not depend on).
 */
const env: Record<string, string | undefined> =
  (globalThis as { process?: { env?: Record<string, string | undefined> } })
    .process?.env ?? {};

/**
 * Resolve the dev-server port from `WAILS_VITE_PORT`.
 *
 * Wails passes the port it expects through this variable, so an unvalidated
 * implicit fallback (`Number(x) || DEFAULT_PORT`) would silently let Vite and
 * the Wails CLI disagree and produce a blank WebView with no clue. Invalid
 * values are therefore logged explicitly before falling back.
 */
function resolveDevPort(): number {
  const raw = env.WAILS_VITE_PORT?.trim();

  if (raw === undefined || raw === "") {
    return DEFAULT_PORT;
  }

  const port = Number(raw);

  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    console.warn(
      `[vite] Invalid WAILS_VITE_PORT "${raw}", falling back to ${DEFAULT_PORT}.`,
    );
    return DEFAULT_PORT;
  }

  return port;
}

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    // Override with WAILS_VITE_HOST=0.0.0.0 for containerized / remote-dev setups.
    host: env.WAILS_VITE_HOST?.trim() || DEFAULT_HOST,
    port: resolveDevPort(),
    strictPort: true,
  },
  plugins: [vue(), wails("./bindings")],
});
