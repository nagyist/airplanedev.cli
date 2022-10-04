const worker = require('@temporalio/worker');
const { writeFile } = require('fs/promises');

// Create a workflow bundle using tooling provided by Temporal;
// see https://docs.temporal.io/docs/typescript/workers/#prebuilt-workflow-bundles
// for details.
async function bundle() {
  const { code } = await worker.bundleWorkflowCode({
    workflowsPath: '/airplane/.airplane/workflow-shim.js',
    workflowInterceptorModules: ['/airplane/.airplane/workflow-interceptors.js'],
    webpackConfigHook: (config) => {
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
      }
    },
  });

  await writeFile('/airplane/.airplane/workflow-bundle.js', code);
}

bundle();
