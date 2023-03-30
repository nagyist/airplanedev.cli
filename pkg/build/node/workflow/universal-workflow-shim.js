import airplane from "airplane";
import {
  proxySinks,
  isCancellation,
  CancellationScope,
} from "@temporalio/workflow";
import { registerWorkflowRuntime } from "@airplane/workflow-runtime/internal";

const taskImports = {
  {{- range $taskImport := .TaskImports}}
  "{{$taskImport.CompiledFile}}": require("../{{$taskImport.UserFile}}"),
  {{- end}}
}

const { logger } = proxySinks();

// Main entrypoint to workflow; wraps a `workflow` function in the user code.
//
// This name must match the name we use when executing the workflow in
// the Airplane API.
export async function __airplaneEntrypoint(params, workflowArgs) {
  registerWorkflowRuntime();
  if (CancellationScope.current().consideredCancelled) {
    logger.internal("airplane_status:cancelled");
    return;
  }
  logger.internal("airplane_status:active");
  try {
    // Monkey patch node globals
    global.process = {
      env: workflowArgs.EnvVars,
    };
    global.console = {
      debug: logger.debug,
      info: logger.info,
      log: logger.log,
      warn: logger.warn,
      error: logger.error,
    };

    const task = taskImports[workflowArgs.Entrypoint][workflowArgs.EntrypointFunc];

    var ret;
    if ("__airplane" in task) {
      ret = await task.__airplane.baseFunc(JSON.parse(params[0]));
    } else {
      ret = await task(JSON.parse(params[0]));
    }
    if (ret !== undefined) {
      airplane.setOutput(ret);
    }
    logger.internal("airplane_status:succeeded");
  } catch (err) {
    logger.info(err);
    logger.internal(JSON.stringify(err));
    if (isCancellation(err)) {
      logger.internal("airplane_status:cancelled");
    } else {
      // Print the error's message directly when possible. Otherwise, it includes the
      // error's name (e.g. "RunTerminationError: ...").
      const message = err instanceof Error ? err.message : String(err);
      logger.info(`airplane_output_set:error ${JSON.stringify(message)}`);
      logger.internal("airplane_status:failed");
    }
  }
}
