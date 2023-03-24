const worker = require("@temporalio/worker");
const { writeFile } = require("fs/promises");
const { builtinModules } = require("module");
const webpack = require("webpack");

// Temporal does not let you use any Node built-in, except for `assert`, which they shim.
const shimmedModules = builtinModules.filter((name) => name !== "assert");

// Create a workflow bundle using tooling provided by Temporal;
// see https://docs.temporal.io/docs/typescript/workers/#prebuilt-workflow-bundles
// for details.
async function bundle() {
  // Generate a fallback for each of the built-in Node modules.
  const fallbacks = {};
  await Promise.all(
    shimmedModules.map(async (moduleName) => {
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
    })
  );

  const { code } = await worker.bundleWorkflowCode({
    workflowsPath: "/airplane/.airplane/workflow-shim.js",
    workflowInterceptorModules: ["/airplane/.airplane/workflow-interceptors.js"],
    webpackConfigHook: (config) => {
      // Temporal aliases each to "false" so they generate as empty modules. We want to replace them
      // with our shim above, so remove those aliases.
      for (let moduleName of shimmedModules) {
        delete config.resolve.alias[moduleName];
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
          },
        },
        plugins: [
          ...(config.plugins || []),
          // Rewrite all `node:*` imports to the corresponding fallback file.
          // We do this with a plugin since `resolve.fallback` does not support import schemes (e.g. `node:`).
          // Based on: https://github.com/webpack/webpack/issues/13290#issuecomment-987880453
          new webpack.NormalModuleReplacementPlugin(/^node:/, (resource) => {
            const moduleName = resource.request.replace(/^node:/, "");
            resource.request = fallbacks[moduleName];
          }),
        ],
      };
    },
  });

  await writeFile("/airplane/.airplane/workflow-bundle.js", code);
}

// Test the bundle by replaying the workflow bundle with fake data.
async function testBundle() {
  const sinks = {
    exporter: {},

    logger: {
      debug: {
        fn: (workflowInfo, message, ...optionalParams) => {
          printConsoleLog(message);
        },
      },
      info: {
        fn: (workflowInfo, message, ...optionalParams) => {
          printConsoleLog(message);
        },
      },
      log: {
        fn: (workflowInfo, message, ...optionalParams) => {
          printConsoleLog(message);
        },
      },
      warn: {
        fn: (workflowInfo, message, ...optionalParams) => {
          printConsoleLog(message);
        },
      },
      error: {
        fn: (workflowInfo, message, ...optionalParams) => {
          printConsoleLog(message);
        },
      },
      internal: {
        fn: (workflowInfo, message, ...optionalParams) => {
          printConsoleLog(message);
        },
      },
      raw: {
        fn: (workflowInfo, message, ...optionalParams) => {
          printConsoleLog(message);
        },
      },
    },
  };

  let workerLogs = [];

  const logger = new worker.DefaultLogger("INFO", ({ level, message, meta }) => {
    let workerLog = "";
    if (meta) {
      workerLog = `[${level}] ${message} ${JSON.stringify(meta)}`;
    } else {
      workerLog = `[${level}] ${message}`;
    }

    printConsoleLog(workerLog);
    workerLogs.push(workerLog);
  });
  const telemetryOptions = {
    logging: {
      forward: {
        level: "INFO",
      },
    },
  };
  worker.Runtime.install({ logger, telemetryOptions });

  await worker.Worker.runReplayHistory(
    {
      workflowBundle: {
        codePath: "/airplane/.airplane/workflow-bundle.js",
      },
      sinks: sinks,
    },
    {
      // Just a single event that represents the workflow execution starting
      events: [
        {
          eventId: "1",
          eventTime: "2022-07-06T00:33:05.000Z",
          eventType: "WorkflowExecutionStarted",
          version: "0",
          taskId: "1056764",
          workflowExecutionStartedEventAttributes: {
            workflowType: { name: "__airplaneEntrypoint" },
            parentWorkflowNamespace: "",
            parentInitiatedEventId: "0",
            taskQueue: { name: "test", kind: "Normal" },
            input: { payloads: [] },
            workflowTaskTimeout: "10s",
            continuedExecutionRunId: "",
            initiator: "Unspecified",
            originalExecutionRunId: "bc761765-7fca-4d3e-89ff-0fa49379dc7a",
            identity: "61356@tester",
            firstExecutionRunId: "bc761765-7fca-4d3e-89ff-0fa49379dc7a",
            attempt: 1,
            cronSchedule: "",
            firstWorkflowTaskBackoff: "0s",
            header: { fields: {} },
          },
        },
      ],
    }
  );

  workerLogs.forEach((value) => {
    const failureIndex = value.indexOf("failure=Failure");
    const stackTraceIndex = value.indexOf("stack_trace:");
    const encodedAttributesIndex = value.indexOf("encoded_attributes:");

    if (failureIndex > 0) {
      if (stackTraceIndex > 0 && encodedAttributesIndex > 0) {
        const stackValue = value.substring(stackTraceIndex + 14, encodedAttributesIndex - 3);
        // Need to be careful about newline escaping here. The easiest way around this is
        // to just refer to the associated characters via their ASCII codes.
        throw new Error(
          `testing workflow bundle: ${stackValue.replaceAll(
            String.fromCharCode(92) + String.fromCharCode(110),
            String.fromCharCode(10)
          )}`
        );
      } else {
        throw new Error(`testing workflow bundle: ${value.substring(failureIndex)}`);
      }
    }
  });
}

function printConsoleLog(message) {
  // Don't show benign and/or noisy logs that will just confuse us and users.
  //
  // TODO: Consider hiding all logs unless there's an error in the testing process.
  if (
    !message.startsWith("TypeError") &&
    !message.startsWith("airplane_") &&
    !message.startsWith("[ERROR] External sink function threw an error") &&
    !message.startsWith("[ERROR] Workflow referenced an unregistered external sink")
  ) {
    console.log(message);
  }
}

async function main() {
  try {
    await bundle();
    await testBundle();
  } catch (error) {
    console.error(error);
    process.exitCode = 1;
  }
}

main();
