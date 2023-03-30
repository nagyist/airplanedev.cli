import airplane from "airplane";

export default airplane.task(
  {
    slug: "my_task",
    resources: {
      my_alias: "alias",
      no_alias: "no_alias"
    }
  },
  async () => {
    return []
  },
);
