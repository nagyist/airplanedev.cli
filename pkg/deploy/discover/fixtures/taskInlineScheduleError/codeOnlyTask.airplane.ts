import airplane from "airplane";

export const collatz = airplane.task(
  {
    slug: "collatz",
    name: "Collatz Conjecture Step",
    parameters: { num: { name: "Num", type: "integer" } },
    schedules: { fooBar: { name: "Foo Bar", cron: "0 0 * * *" } },
  },
  () => {}
);
