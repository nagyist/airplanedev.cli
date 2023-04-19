import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "path";
import replace from "@rollup/plugin-replace";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    // Vite automatically replaces process.env with {} which breaks accessing env vars.
    // This plugin replaces process.env with process['env'] to work around this.
    replace({
      "process.env.": "process['env'].",
      preventAssignment: true,
      // By default, this plugin will only replace on word boundaries. We want to replace process.env even if it's a
      // part of a larger expression.
      delimiters: ["", ""],
      exclude: ["**/node_modules/**"],
    }),
    react(),
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
