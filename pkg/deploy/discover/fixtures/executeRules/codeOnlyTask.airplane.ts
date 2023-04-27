import airplane from "airplane";

export const collatz = airplane.task(
  {
    slug: "collatz",
    name: "Collatz Conjecture Step",
    allowSelfApprovals: false,
    concurrencyKey: "key",
    concurrencyLimit: 2,
  },
  () => {}
);
