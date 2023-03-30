import airplane from "airplane";

export const myNumber = 10;

export const myString = "Hello World!";

export const myNull = null;

export const myUndefined = undefined;

export default async function (params) {
  airplane.setOutput(params.id);
}
