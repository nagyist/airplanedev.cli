import airplane from "airplane";

// Converted from runbook "existing_runbook" (id: rbk1234)
// via `airplane task init --from-runbook existing_runbook`
export default airplane.task(
  {
    slug: "existing_runbook",
    name: "Existing runbook (task)",
    parameters: {},
    resources: ["db", "rest"],
    // Optionally uncomment the following line to enable the workflow runtime.
    // The workflow runtime can run for significantly longer than the default
    // runtime, but it comes with a few restrictions. For more information,
    // see: https://docs.airplane.dev/tasks/runtimes
    //
    // runtime: "workflow"
  },
  async (params) => {
    const sql = await airplane.sql.query<any>("db", "SELECT 1");

    const rest = await airplane.rest.request<any>("rest", "GET", "/");

    // Optionally return output that will be rendered in the run UI: https://docs.airplane.dev/tasks/output
    // return {}
  }
);
