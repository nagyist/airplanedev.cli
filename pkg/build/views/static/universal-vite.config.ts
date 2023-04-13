import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "path";
import { replaceCodePlugin } from "vite-plugin-replace";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    // Vite automatically replaces process.env with {} which breaks accessing env vars.
    // This plugin replaces process.env with process['env'] to work around this.
    replaceCodePlugin({
      replacements: [
        {
          from: /process\.env/g,
          to: "process['env']",
        },
      ],
    }),
  ],
  envPrefix: "AIRPLANE_",
  resolve: {
    preserveSymlinks: true,
  },
  build: {
    rollupOptions: {
      input: {
        {{- range $entrypoint := .Entrypoints}}
          "{{$entrypoint}}": resolve(__dirname, "{{$entrypoint}}/index.html"),
        {{- end}}
      },
    },
    assetsDir: "",
    chunkSizeWarningLimit: 5000,
    sourcemap: true,
  },
  base: "",
});
