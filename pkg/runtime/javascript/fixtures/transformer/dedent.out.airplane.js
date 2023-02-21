import airplane from "airplane";

export default airplane.task(
  {
    slug: "my_task",
    description: `
      An updated description that spans a few lines:

      - Attempt 1
      - Attempt 2`
  },
  async () => {
    return []
  },
);
