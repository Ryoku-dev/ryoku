import { defineConfig, type Plugin } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "node:path";
import { fileURLToPath } from "node:url";

// The chat reducer is the shell's (ryoku/shell/.../lib/chatstate.js), written
// as plain script for Quickshell's JS engine. It carries no ESM exports, so
// this plugin appends them when Vite loads it: one source of truth for the
// protocol, no copy under web/.
const chatStateFile = fileURLToPath(
  new URL("../../shell/quickshell/shell/services/lib/chatstate.js", import.meta.url),
);

function sharedReducer(): Plugin {
  return {
    name: "rashin-shared-chatstate",
    transform(code, id) {
      if (id !== chatStateFile) return null;
      return { code: code + "\nexport { initialState, applyEvent };\n", map: null };
    },
  };
}

// RASHIN_DEV_TARGET points the dev server at a daemon; the default is the
// user's live one on 3600, so pass an isolated preview (3611) when a change
// sends chat turns.
const brandDir = fileURLToPath(new URL("../../assets/brand", import.meta.url));
const target = process.env.RASHIN_DEV_TARGET ?? "http://127.0.0.1:3600";

export default defineConfig({
  plugins: [sharedReducer(), svelte()],
  base: "./",
  resolve: {
    alias: {
      $lib: fileURLToPath(new URL("./src/lib", import.meta.url)),
      $chatstate: chatStateFile,
      // The brand marks live with the rest of Ryoku's brand (ryoku/assets/brand);
      // the console inlines them at build time rather than keeping a copy.
      $brand: brandDir,
    },
  },
  server: {
    port: 5173,
    fs: { allow: [".", brandDir, path.dirname(chatStateFile)] },
    proxy: {
      // The daemon's Prowl proxy only answers its own origin, so the dev
      // server presents the daemon's host and drops the browser's Origin.
      "/api": {
        target,
        changeOrigin: true,
        configure: (proxy) => proxy.on("proxyReq", (req) => req.removeHeader("origin")),
      },
      "/ws": { target: target.replace(/^http/, "ws"), ws: true },
    },
  },
  build: {
    outDir: "../backend/web/dist",
    emptyOutDir: true,
    target: "safari17",
    sourcemap: false,
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        // The effect runtimes and the component library change on their own
        // schedule; hashing them apart keeps a console update small.
        manualChunks: {
          fx: ["react", "react-dom", "border-beam", "thinking-orbs/engine", "blobatar", "@blobatar/svelte"],
          bits: ["bits-ui"],
        },
      },
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
