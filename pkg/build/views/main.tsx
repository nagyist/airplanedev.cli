import {
  Container,
  ViewProvider,
  setEnvVars,
  ErrorBoundary,
} from "@airplane/views";
import React from "react";
import ReactDOM from "react-dom/client";
import App from "{{.Entrypoint}}";

const paramsEnvSlug = typeof window !== "undefined" ? (new URL(window.location)).searchParams.get("__env") : null;

setEnvVars(
  import.meta.env.AIRPLANE_API_HOST || "https://api.airplane.dev",
  import.meta.env.AIRPLANE_TOKEN,
  import.meta.env.AIRPLANE_API_KEY,
  paramsEnvSlug || import.meta.env.AIRPLANE_ENV_SLUG
);
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ViewProvider>
      <ErrorBoundary>
        <Container p="xl">
          <App />
        </Container>
      </ErrorBoundary>
    </ViewProvider>
  </React.StrictMode>
);
