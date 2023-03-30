import airplane from "airplane"

export default airplane.task(
    {
        slug: "default_export_subfolder",
        name: "Default Export Subfolder",
        nodeVersion: "16",
    },
    () => {
        airplane.setOutput("running:default_export_subfolder");
    }
)
