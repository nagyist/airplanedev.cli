import airplane from "airplane";

export const myTask = airplane.task({
    slug: "my_task",
    name: "My Task",
    parameters: { num: { name: "Num", type: "integer" } },
}, () => {});
