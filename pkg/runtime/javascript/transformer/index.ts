import { exit } from "node:process";
import { transform } from "./transformer";

const run = async () => {
  try {
    const file = process.argv[2];
    if (!file) {
      throw new Error("a file arg is not set");
    }
    const slug = process.argv[3];
    if (!slug) {
      throw new Error("a slug arg is not set");
    }
    const defSerialized = process.argv[4];
    if (!defSerialized) {
      throw new Error("a definition arg is not set");
    }
    const def = JSON.parse(defSerialized);

    await transform(file, slug, def);
  } catch (err: any) {
    console.error(err);
    if ("message" in err) {
      console.error("__airplane_error " + JSON.stringify(err.message));
    }
    exit(1);
  }
};

run();
