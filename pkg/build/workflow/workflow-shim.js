import airplane from 'airplane';
import { proxySinks, CancelledFailure } from '@temporalio/workflow';
import task from '{{.Entrypoint}}';

const { logger } = proxySinks();

// Main entrypoint to workflow; wraps a `workflow` function in the user code.
//
// This name must match the name we use when executing the workflow in
// the Airplane API.
export async function __airplaneEntrypoint(params, workflowArgs) {
  logger.info('airplane_status:active');
  try {
    // Monkey patch process.env
    global.process = {
      env: workflowArgs.EnvVars
    }

    var result = await task(JSON.parse(params[0]));
    if (result !== undefined) {
      airplane.setOutput(result);
    }

    logger.info('airplane_status:succeeded');
  } catch (err) {
    logger.info(err);
    if (err instanceof CancelledFailure) {
      logger.info(`airplane_output_set:error ${JSON.stringify("Workflow cancelled")}`);
      logger.info('airplane_status:cancelled');
    } else {
      logger.info(`airplane_output_set:error ${JSON.stringify(String(err))}`);
      logger.info('airplane_status:failed');
    }
  }
}
