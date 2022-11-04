import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      input: {
        {{- range $entrypoint := .Entrypoints}}
          "{{$entrypoint}}": resolve(__dirname, "views/{{$entrypoint}}/index.html"),
        {{- end}}
      },
    },
  },
});
