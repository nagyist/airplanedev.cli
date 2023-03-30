import react from "@vitejs/plugin-react";
import { defineConfig, searchForWorkspaceRoot } from "vite";

export default defineConfig({
  plugins: [react()],
  envPrefix: "AIRPLANE_",
  resolve: {
    preserveSymlinks: true,
  },
  base: "{{.Base}}",
  build: {
    assetsDir: "",
    sourcemap: true,
  },
  server: {
    fs: {
      allow: [
        searchForWorkspaceRoot(process.cwd()),
        // If base is non-empty, Vite attempts to serve files from a subpath of the actual workspace root (i.e.
        // {workspace}/{base}. We do not want that, and are simply using base to proxy Studio requests to the vite
        // server, and so we allow the original workspace root (equivalent to the Airplane project root).
        "{{.Root}}",
      ],
    },
  },
});
