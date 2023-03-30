import airplane from "airplane"

export default airplane.task(
    {
        slug: "default_export_root_folder",
        name: "Default Export Root Folder",
        nodeVersion: "16",
    },
    () => {
        return "running:default_export_root_folder";
    }
)

export const named = airplane.task(
    {
        slug: "named_export_root_folder",
        name: "Named Export Root Folder",
        nodeVersion: "16",
    },
    () => {
        return "running:named_export_root_folder";
    }
)
