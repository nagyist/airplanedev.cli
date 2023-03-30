This folder contains a shim for developing and building a view.

- `main.tsx` The entrypoint of the view. Here we add required providers, error boundaries, and UI wrappers.
- `index.html` The html in which the view is rendered. See https://vitejs.dev/guide/#index-html-and-project-root
- `vite.config.ts` The vite config for developinga and building a view.

## Testing locally

Additional files have been included so the shim can be tested locally.

1. Go to `main.tsx`.
1. Comment out `import App from "{{.Entrypoint}}";`.
1. Change `<App />` -> `<div>hi</div>` or any other valid view code. You can import views components and/or copy and paste a view in from elsewhere to see how it will look.
1. Run `yarn dev`.
