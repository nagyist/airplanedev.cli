import airplane from "airplane";

export default airplane.task(
  {
    slug: "my_task_2",
    name: "Task name",
    description: "Task description",
    parameters: {
      dry: {
        name: "Dry run?",
        description: "Whether or not to run in dry-run mode.",
        type: "boolean",
        required: false,
        default: true
      }
    },
    runtime: "workflow",
    resources: ["db"],
    envVars: {
      AWS_ACCESS_KEY: {
        config: "aws_access_key"
      }
    },
    timeout: 60,
    constraints: {
      cluster: "k8s",
      vpc: "tasks"
    },
    requireRequests: true,
    allowSelfApprovals: false,
    schedules: {
      daily: {
        name: "Daily",
        description: "Runs every day at 12 UTC",
        cron: "0 12 * * *",
        paramValues: {
          dry: false
        }
      }
    }
  },
  async () => {
    return []
  },
);
