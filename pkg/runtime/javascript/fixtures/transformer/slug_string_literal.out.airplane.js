import airplane from "airplane";

export default airplane.task(
  {
    slug: "my_task",
    name: "This task is mine"
  },
  async () => {
    return []
  },
);
