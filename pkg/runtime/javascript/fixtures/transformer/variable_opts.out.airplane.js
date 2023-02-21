import airplane from "airplane";

const opts = {
  slug: "my_task",
  name: "This task is mine"
}

export default airplane.task(
  opts,
  async () => {
    return []
  },
);
