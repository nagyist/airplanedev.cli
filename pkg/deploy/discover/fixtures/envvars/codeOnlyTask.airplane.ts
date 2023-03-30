import airplane from "airplane";

export const collatz = airplane.task(
  {
    slug: "collatz",
    name: "Collatz Conjecture Step",
    envVars: { ENV1: "1", ENV3: "3a" },
  },
  () => {}
);
