import { NativeConnection, Worker } from '@temporalio/worker';

// Activity code runs in the same node process as the worker, so we import it here directly.
import { createActivities } from "@airplane/workflow-runtime/internal";

function formatWorkflowLog(source, workflowInfo, message) {
  return `airplane_workflow_log:${source}//${workflowInfo.workflowId}/${workflowInfo.runId} ${message}`
}

// Main worker entrypoint; starts a worker that will process activities
// and workflows for a single task queue (equivalent to airplane task revision).
async function runWorker(params) {
  // Get temporal host, queue, and namespace from the environment.
  const temporalHost = process.env.AP_TEMPORAL_HOST;
  if (temporalHost === undefined) {
    throw 'AP_TEMPORAL_HOST is not set in environment';
  }

  const taskQueue = process.env.AP_TASK_QUEUE;
  if (taskQueue === undefined) {
    throw 'AP_TASK_QUEUE is not set in environment';
  }

  const namespace = process.env.AP_NAMESPACE;
  if (namespace === undefined) {
    throw 'AP_NAMESPACE is not set in environment';
  }

  const temporalToken = process.env.AP_TEMPORAL_TOKEN;
  if (temporalToken === undefined) {
    throw 'AP_TEMPORAL_TOKEN is not set in environment';
  }

  // We use TLS when hitting a remote Temporal API (i.e., behind a load balancer),
  // but not a local one. The easiest way to tell the difference is by
  // looking at the port.
  const useTLS = temporalHost.endsWith(':443');

  console.log(
    `Starting worker with temporal host ${temporalHost}, task queue ${taskQueue}, namespace ${namespace}, useTLS ${useTLS}`
  );

  const connection = await NativeConnection.create({
    address: temporalHost,
    metadata: {
      authorization: temporalToken,
    },
    tls: useTLS,
  });

  // Sinks allow us to log from workflows.
  const sinks = {
    // We prefix all logs with the Temporal workflow ID (equivalent to the Airplane run ID) and the Temporal run ID so
    // we can link the logs back to specific Airplane and Temporal runs.
    logger: {
      debug: {
        fn(workflowInfo, message) {
          console.debug(formatWorkflowLog("workflow", workflowInfo, message));
        },
      },
      info: {
        fn(workflowInfo, message) {
          console.info(formatWorkflowLog("workflow", workflowInfo, message));
        },
      },
      log: {
        fn(workflowInfo, message) {
          console.log(formatWorkflowLog("workflow", workflowInfo, message));
        },
      },
      warn: {
        fn(workflowInfo, message) {
          console.warn(formatWorkflowLog("workflow", workflowInfo, message));
        },
      },
      error: {
        fn(workflowInfo, message) {
          console.error(formatWorkflowLog("workflow", workflowInfo, message));
        },
      },
      // logger.internal is used only in workflow-shim.js to differentiate between logs from the user's code and from
      // our own shim.
      internal: {
        fn(workflowInfo, message) {
          console.log(formatWorkflowLog("shim", workflowInfo, message));
        },
      },
      // logger.raw does not automatically prefix the message with airplane_workflow_log, and is mainly used in
      // contexts where console.log is overridden and we want to set a custom prefix.
      raw: {
        fn(workflowInfo, message) {
          console.log(message);
        },
      },
    },
  };

  const worker = await Worker.create({
    // Path to bundle created by bundle-workflow.js script; this should be relative
    // to the shim.
    workflowBundle: {
      codePath: '/airplane/.airplane/workflow-bundle.js',
      sourceMapPath: '/airplane/.airplane/workflow-bundle.map.js',
    },
    activities: {
      ...createActivities(),
    },
    connection,
    namespace,
    taskQueue,
    interceptors: {
      activityInbound: [(ctx) => new ActivityLogInboundInterceptor(ctx)],
    },
    sinks,
  });

  await worker.run();
}

// Interceptor that allows us to add extra logs around when activities start and
// end. See https://docs.temporal.io/docs/typescript/interceptors for details.
export class ActivityLogInboundInterceptor {
  info;
  constructor(ctx) {
    this.info = ctx.info;
  }

  async execute(input, next) {
    activityLog(this.info, `Activity execution starting: ${JSON.stringify(input)}`);
    try {
      const result = await next(input);
      activityLog(this.info, `Activity execution result: ${JSON.stringify(result)}`);
      return result;
    } catch (error) {
      activityLog(this.info, `Activity execution errored: ${error}`);
      throw error;
    }
  }
}

function activityLog(info, message) {
  // Prefix all logs with metadata that we can use to link the message back to a
  // specific task run.
  console.log(
    `airplane_workflow_log:activity/${info.activityType}/${info.workflowExecution.workflowId}/${info.workflowExecution.runId} ${message}`
  );
}

// This function is the worker's entrypoint.
async function main() {
  if (process.argv.length !== 3) {
    console.error(`Expected to receive a single argument (via {{ "{{JSON}}" }}). Task CLI arguments may be misconfigured.`);
    process.exit(1);
  }

  try {
    await runWorker(JSON.parse(process.argv[2]));
  } catch (err) {
    console.error(`Worker errored: ${err}`);
    process.exit(1);
  }
}

main();
