import airplane from "airplane";

export default airplane.task(
  {
    slug: "default_export_subfolder",
    name: "Default",
    runtime: "workflow",
  },
  () => {
    return "running:default_export_subfolder";
  }
);
