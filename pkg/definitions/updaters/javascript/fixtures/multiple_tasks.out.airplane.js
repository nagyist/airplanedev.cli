import airplane from "airplane";

export const myTask = airplane.task(
  {
    slug: "my_task",
    name: "My task"
  },
  async () => {
    return []
  },
);

export const myTask2 = airplane.task(
  {
    slug: "my_task_two",
    name: "My task (v2)"
  },
  async () => {
    return []
  },
);

export const myView = airplane.view(
  {
    slug: "my_view",
    name: "My View"
  },
  async () => {
    return []
  },
);
