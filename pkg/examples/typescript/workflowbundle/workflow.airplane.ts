import airplane from "airplane";

export default airplane.workflow(
  {
    slug: "default_export_root_folder",
    name: "Default",
  },
  () => {
    return "running:default_export_root_folder";
  }
);

export const named = airplane.workflow(
  {
    slug: "named_export_root_folder",
    name: "Named",
  },
  () => {
    return "running:named_export_root_folder";
  }
);
