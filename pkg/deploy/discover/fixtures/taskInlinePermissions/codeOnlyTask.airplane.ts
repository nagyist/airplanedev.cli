import airplane from "airplane";

export const collatz = airplane.task(
  {
    slug: "collatz",
    name: "Collatz Conjecture Step",
    permissions: {
      viewers: {
        groups: ["group1"],
        users: ["user1"],
      },
      requesters: {
        groups: ["group2"],
      },
      executers: {
        groups: ["group3", "group4"],
      },
      admins: {
        groups: ["group5"],
      },
    },
  },
  () => {}
);
