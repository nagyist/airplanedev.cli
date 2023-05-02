import airplane from "airplane";

export default airplane.task(
  {
    slug: "my_task",
    permissions: "team_access"
  },
  async () => {
    return []
  },
);
