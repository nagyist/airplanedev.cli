import airplane from "airplane";

export default airplane.workflow(
  {
    slug: "default_export_subfolder",
    name: "Default",
  },
  () => {
    return "running:default_export_subfolder";
  }
);
