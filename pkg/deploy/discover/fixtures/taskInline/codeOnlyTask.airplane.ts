import airplane from "airplane";

export const myNumber = 10;

export const myString = "Hello World!";

export const myNull = null;

export const myUndefined = undefined;

export const collatz = airplane.task(
  {
    slug: "collatz",
    name: "Collatz Conjecture Step",
    parameters: { num: { name: "Num", type: "integer" } },
    defaultRunPermissions: "task-participants",
    permissions: {},
  },
  () => {}
);
