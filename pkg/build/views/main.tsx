import {
  Container,
  ViewProvider,
  setEnvVars,
  ErrorBoundary,
} from "@airplane/views";
import React from "react";
import ReactDOM from "react-dom/client";
import App from "{{.Entrypoint}}";

const isInStudio = Boolean({{.IsInStudio}});

// Polyfill process.env.
const global =
  (typeof globalThis !== 'undefined' && globalThis) ||
  (typeof self !== 'undefined' && self) ||
  (typeof global !== 'undefined' && global) ||
  {}
global.process = global.process || {};
global.process.env = global.process.env || {};

/**
 * Gets the value of a query parameter by key.
 * If the view is loaded in an iframe, this will get the query param from the iframe.
 */
const getQueryParam = (key: string) => {
  if (typeof window === "undefined") {
    return "";
  }
  return new URL(window.location.href).searchParams.get(key) || "";
};

/**
 * Gets the value of all query params.
 * If the view is loaded in an iframe, this will get the query param from the iframe.
 */
const getAllQueryParams = () => {
  if (typeof window === "undefined") {
    return new URLSearchParams().entries();
  }
  return new URL(window.location.href).searchParams.entries();
};

const camelToSnakeCase = (str: string) => str.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`).toUpperCase(); 

// Plumb environment variables that are passed into the view -> @airplane/views.
setEnvVars(
  import.meta.env.AIRPLANE_API_HOST || "https://api.airplane.dev",
  import.meta.env.AIRPLANE_TOKEN,
  import.meta.env.AIRPLANE_API_KEY,
  getQueryParam("__env") || import.meta.env.AIRPLANE_ENV_SLUG
);

// Set runtime environment variables.
for (const [key, value] of getAllQueryParams()) {
  if (key.startsWith("__")) {
    const camel = key.slice(2);
    let title = camelToSnakeCase(camel);
    let val = value;
    if (isInStudio) {
      if (camel === "envSlug" || camel === "envId" || camel === "envName" || camel==="env") {
        val = "studio";
      }
      if (camel === "envIsDefault") {
        val = "true";
      }
    }

    if (!title.startsWith("AIRPLANE_")) {
      title = `AIRPLANE_${title}`;
    }

    process.env[title] = val;
  } else {
    process.env[key] = value;
  }
}

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
