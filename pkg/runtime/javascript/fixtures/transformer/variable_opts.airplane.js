import airplane from "airplane";

const opts = {
  slug: "my_task",
  name: "My task"
}

export default airplane.task(
  opts,
  async () => {
    return []
  },
);
