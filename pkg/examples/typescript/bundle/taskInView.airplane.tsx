import airplane from "airplane";
import { Text } from "@airplane/views";

export default airplane.task(
  {
    slug: "in_view",
    name: "Default Export Root Folder",
    nodeVersion: "16",
  },
  () => {
    airplane.setOutput("running:in_view");
  }
);

export const named = airplane.view(
  {
    slug: "named_export_root_folder",
    name: "Named Export Root Folder",
  },
  () => <Text>hi</Text>
);
