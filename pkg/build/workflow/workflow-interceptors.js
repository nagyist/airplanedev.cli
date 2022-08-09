import { workflowInfo } from '@temporalio/workflow';
import { proxySinks } from '@temporalio/workflow';
const { logger } = proxySinks();

// Interceptor that allows us to log outbound client calls made from workflow.
// See https://docs.temporal.io/docs/typescript/interceptors for details.
class WorkflowLogOutboundInterceptor {
  info;
  constructor(info) {
    this.info = info;
  }

  async scheduleActivity(input, next) {
    const activityType = input.activityType;

    workflowLog(this.info, `Scheduling activity ${activityType}: ${JSON.stringify(input)}`);
    try {
      const result = await next(input);
      workflowLog(
        this.info,
        `Activity ${activityType} result: ${JSON.stringify(result)}`
      );
      return result;
    } catch (error) {
      workflowLog(this.info, `Activity ${activityType} errored: ${error}`);
      throw error;
    }
  }

  async scheduleLocalActivity(input, next) {
    workflowLog(this.info, `Scheduling local activity: ${JSON.stringify(input)}`);
    try {
      const result = await next(input);
      workflowLog(this.info, `Local activity result: ${JSON.stringify(result)}`);
      return result;
    } catch (error) {
      workflowLog(this.info, `Local activity errored: ${error}`);
      throw error;
    }
  }

  async startTimer(input, next) {
    workflowLog(this.info, `Starting timer: ${JSON.stringify(input)}`);
    try {
      const result = await next(input);
      workflowLog(this.info, `Starting timer result: ${JSON.stringify(result)}`);
      return result;
    } catch (error) {
      workflowLog(this.info, `Timer errored: ${error}`);
      throw error;
    }
  }
}

// Interceptor that allows us to log inbound client calls made to workflow.
// See https://docs.temporal.io/docs/typescript/interceptors for details.
class WorkflowLogInboundInterceptor {
  info;
  constructor(info) {
    this.info = info;
  }

  async execute(input, next) {
    workflowLog(this.info, `Workflow execution starting: ${JSON.stringify(input)}`);
    try {
      const result = await next(input);
      workflowLog(
        this.info,
        `Workflow execution result: ${JSON.stringify(result)}`
      );
      return result;
    } catch (error) {
      workflowLog(this.info, `Workflow execution errored: ${error}`);
      throw error;
    }
  }

  async handleSignal(input, next) {
    workflowLog(this.info, `Handling signal: ${JSON.stringify(input)}`);
    try {
      const result = await next(input);
      workflowLog(this.info, `Signal result: ${JSON.stringify(result)}`);
      return result;
    } catch (error) {
      workflowLog(this.info, `Error handling signal: ${error}`);
      throw error;
    }
  }
}

// Need to export interceptors in this format so that Temporal can
// find them.
export const interceptors = () => ({
  outbound: [new WorkflowLogOutboundInterceptor(workflowInfo())],
  inbound: [new WorkflowLogInboundInterceptor(workflowInfo())],
});

function workflowLog(info, message) {
  // Log out interceptor messages with specific prefix so that we can link them back to a specific task run. Note that
  // we cannot directly call console.log here because we override the console.log method in the workflow shim, which
  // would cause the output to get unnecessarily prepended with airplane_workflow_log:workflow
  logger.raw(`airplane_workflow_log:interceptor//${info.workflowId}/${info.runId} ${message}`);
}
