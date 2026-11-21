import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { svelteTesting } from "@testing-library/svelte/vite";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  // svelteTesting makes Svelte resolve to its browser build and removes what a test rendered when it ends. It is
  // only for the tests: the app's own build does not load it. Most tests run in Node; a test that needs a DOM says so
  // on its first line (`// @vitest-environment happy-dom`).
  plugins: [svelte(), wails("./bindings"), ...(process.env.VITEST ? [svelteTesting()] : [])],
});
