import airplane from "airplane";

export default airplane.task(
  {
    slug: "my_task",
    name:
      // Some comments.
      "This task is mine",
  },
  async () => {
    return []
  },
);
