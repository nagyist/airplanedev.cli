const worker = require('@temporalio/worker');
const { writeFile } = require('fs/promises');
const { builtinModules } = require('module');

// Temporal does not let you use any Node built-in, except for `assert`, which they shim.
const shimmedModules = builtinModules.filter(name => name !== "assert")

// Create a workflow bundle using tooling provided by Temporal;
// see https://docs.temporal.io/docs/typescript/workers/#prebuilt-workflow-bundles
// for details.
async function bundle() {
  // Generate a fallback for each of the built-in Node modules.
  const fallbacks = {}
  await Promise.all(shimmedModules.map(async (moduleName) => {
    const shim = `const getError = (path = "${moduleName}") => {
  return new Error(\`Workflows do not have access to Node.js built-in packages (cannot access "\${path}"). Move that logic into a Node.js task and call the task from your workflow.\`);
}

function proxyTarget() {
  throw getError();
}

const proxy = new Proxy(proxyTarget, {
  // Handles accessing fields on the import, e.g. "fs.existsSync"
  get: (target, property, receiver) => {
    // webpack's getDefaultExport function accesses this field when booting up to check
    // how to access the default export. This is not caused by user code, so don't error.
    if (property === "__esModule") {
      return false;
    }
    throw getError(\`${moduleName}.\${property}\`)
  },
  // Handles applying the default module export as a function
  apply: () => { throw getError(); },
  // Handles calling a constructor on the default module export
  construct: () => { throw getError(); },
});

module.exports = proxy;`;
    fallbacks[moduleName] = `/airplane/.airplane/shims-${moduleName.replace(/\//g, "-")}.js`;
    await writeFile(fallbacks[moduleName], shim);
  }));

  const { code } = await worker.bundleWorkflowCode({
    workflowsPath: '/airplane/.airplane/workflow-shim.js',
    workflowInterceptorModules: ['/airplane/.airplane/workflow-interceptors.js'],
    webpackConfigHook: (config) => {
      // Temporal aliases each to "false" so they generate as empty modules. We want to replace them
      // with our shim above, so remove those aliases.
      for (let moduleName of shimmedModules) {
        delete config.resolve.alias[moduleName]
      }
      return {
        ...config,
        // Temporal registers a function here that looks for non-deterministic imports:
        // https://github.com/temporalio/sdk-typescript/blob/3be5ab7b2702c82375390092d371a7463d488ba3/packages/worker/src/workflow/bundler.ts#L180-L194
        //
        // We disable this by overriding it to `undefined`.
        //
        // We choose to do so because we want to be able to import tasks defined with
        // `airplane.task` to call them from workflows. The task likely uses packages
        // that are considered non-deterministic, but the task is not executed within
        // the workflow. Similarly, we want to be able to import helpers that may be
        // in the same file as utilities which use non-deterministic packages. In either
        // case, as long as those packages are not imported from the workflow, we should
        // not error.
        externals: undefined,
        resolve: {
          ...config.resolve,
          fallback: {
            ...config.resolve.fallback,
            ...fallbacks,
          }
        }
      }
    },
  });

  await writeFile('/airplane/.airplane/workflow-bundle.js', code);
}

bundle();
