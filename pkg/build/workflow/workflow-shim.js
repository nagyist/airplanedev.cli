import airplane from 'airplane';
import { proxySinks, CancelledFailure } from '@temporalio/workflow';
import task from '{{.Entrypoint}}';

const { logger } = proxySinks();

// Main entrypoint to workflow; wraps a `workflow` function in the user code.
//
// This name must match the name we use when executing the workflow in
// the Airplane API.
export async function __airplaneEntrypoint(params, workflowArgs) {
  logger.internal('airplane_status:active');
  try {
    // Monkey patch node globals
    global.process = {
      env: workflowArgs.EnvVars
    }
    global.console = {
      debug: logger.debug,
      info: logger.info,
      log: logger.log,
      warn: logger.warn,
      error: logger.error
    }

    var result = await task(JSON.parse(params[0]));
    if (result !== undefined) {
      airplane.setOutput(result);
    }

    logger.internal('airplane_status:succeeded');
  } catch (err) {
    logger.internal(err);
    if (err instanceof CancelledFailure) {
      logger.internal(`airplane_output_set:error ${JSON.stringify("Workflow cancelled")}`);
      logger.internal('airplane_status:cancelled');
    } else {
      logger.internal(`airplane_output_set:error ${JSON.stringify(String(err))}`);
      logger.internal('airplane_status:failed');
    }
  }
}
