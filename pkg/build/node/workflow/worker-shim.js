import { runWorker } from "@airplane/workflow-runtime/internal";

// This function is the worker's entrypoint.
async function main() {
  try {
    await runWorker("/airplane/.airplane/workflow-bundle.js");
  } catch (err) {
    console.error(`Worker errored: ${err}`);
    process.exit(1);
  }
}

main();
