import { exit } from "node:process";
import { transform } from "./transformer";

/**
 * After changing this file, run `yarn build` to bundle into a JS file. Or
 * use `yarn build:watch` to rebuild after every change.
 *
 * To learn more about how to use recast, see their README for docs:
 * - https://github.com/benjamn/recast (parsing/printing logic)
 * - https://github.com/benjamn/ast-types (core "types" like "ObjectExpression")
 *
 * You can also visually explore the parsed AST:
 * - https://astexplorer.net/#/gist/39078d85ea26b8553d533aeb7c235c9f/4858d0d7ccff7b0c3bacc9ea2aff8ff395601518
 *
 * The recast pretty-printer has some outdated styling opinions. We forked recast so we can
 * tweak these behaviors.
 *
 * 1. If an object parameter has multiple lines, a newline will be inserted above and below it.
 *    See: https://github.com/benjamn/recast/issues/228
 *    Fix: https://github.com/airplanedev/recast/pull/1
 */
const run = async () => {
  try {
    const command = process.argv[2];
    if (!command) {
      throw new Error("a command arg is not set");
    }

    const file = process.argv[3];
    if (!file) {
      throw new Error("a file arg is not set");
    }
    const slug = process.argv[4];
    if (!slug) {
      throw new Error("a slug arg is not set");
    }

    if (command === "can_transform") {
      let ok = true;
      try {
        await transform(file, slug, {}, { dryRun: true });
      } catch (err) {
        console.error(String(err));
        ok = false;
      }
      console.log(`__airplane_output ${ok}`);
    } else if (command === "transform") {
      const defSerialized = process.argv[5];
      if (!defSerialized) {
        throw new Error("a definition arg is not set");
      }
      const def = JSON.parse(defSerialized);

      await transform(file, slug, def, { dryRun: false });
    } else {
      throw new Error(`Unknown command: ${command}`);
    }
  } catch (err: any) {
    console.error(err);
    if ("message" in err) {
      console.error("__airplane_error " + JSON.stringify(err.message));
    }
    exit(1);
  }
};

run();
