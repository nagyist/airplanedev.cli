import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
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
  },
  base: "",
});
