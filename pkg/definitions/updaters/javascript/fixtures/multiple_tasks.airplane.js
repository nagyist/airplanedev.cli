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
    slug: "my_task_2",
    name: "My second task"
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
