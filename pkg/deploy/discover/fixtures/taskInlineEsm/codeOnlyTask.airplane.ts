import airplane from "airplane";
import { isEqual } from "lodash-es";

export const collatz = airplane.task(
  {
    slug: "collatz",
    name: "Collatz Conjecture Step",
    parameters: { num: { name: "Num", type: "integer" } },
  },
  () => {
    console.log(isEqual("foo", "bar"));
  }
);
