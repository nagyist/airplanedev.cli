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
      },
      datetime: {
        type: "datetime",
        required: false
      }
    },
    runtime: "workflow",
    resources: ["db"],
    envVars: {
      CONFIG: {
        config: "aws_access_key"
      },
      VALUE: {
        value: "Hello World!"
      }
    },
    timeout: 60,
    constraints: {
      cluster: "k8s",
      vpc: "tasks"
    },
    requireRequests: true,
    allowSelfApprovals: false,
    restrictCallers: ["view", "task"],
    schedules: {
      all: {
        name: "All fields",
        description: "A description",
        cron: "0 12 * * *",
        paramValues: {
          datetime: new Date("2006-01-02T15:04:05Z07:00")
        }
      },
      min: {
        cron: "* * * * *"
      }
    }
  },
  async () => {
    return []
  },
);
