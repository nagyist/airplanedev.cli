import {
  Container,
  ViewProvider,
  setEnvVars,
  ErrorBoundary,
  useRouter,
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
 * ENV_VAR_BLOCK_LIST is the list of query params we do not want to set as environment variables.
 * since the user doesn't need to access them. 
*/
const RUNTIME_ENV_VAR_BLOCK_LIST = ["AIRPLANE_SANDBOX_TOKEN", "AIRPLANE_VIEW_TOKEN"];

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
  getQueryParam("__env") || import.meta.env.AIRPLANE_ENV_SLUG,
  {
    AIRPLANE_TUNNEL_TOKEN: import.meta.env.AIRPLANE_TUNNEL_TOKEN,
    AIRPLANE_SANDBOX_TOKEN: getQueryParam("__airplane_sandbox_token"),
    AIRPLANE_VIEW_TOKEN: getQueryParam("__viewToken"),
  }
);

// Set runtime environment variables.
for (const [key, value] of getAllQueryParams()) {
  let title = key
  let val = value;
  if (key.startsWith("__")) {
    const camel = key.slice(2);
    title = camelToSnakeCase(camel);
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
  } 

  if (!RUNTIME_ENV_VAR_BLOCK_LIST.includes(title)) {
    process.env[title] = val;
  }
}

const AppWrapper = () => {
  const { params } = useRouter();
  return <App params={params} />;
};

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ViewProvider>
      <ErrorBoundary>
        <Container p="xl">
          <AppWrapper />
        </Container>
      </ErrorBoundary>
    </ViewProvider>
  </React.StrictMode>
);
