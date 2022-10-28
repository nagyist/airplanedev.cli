// This file includes a shim that will execute your task code.
import airplane from "airplane";

async function main() {
  if (process.argv.length !== 5) {
    console.log(`airplane_output_set:error ${JSON.stringify(`Expected to receive entrypoint, entrypointFunc, and params (via {{ "{{JSON}}" }}). Task CLI arguments may be misconfigured.`)}`);
    process.exit(1);
  }

  // TODO: needs environment variables to be set properly
  var task = require(process.argv[2])[process.argv[3]]

  try {
    var ret;
    if ("__airplane" in task) {
      ret = await task.__airplane.baseFunc(JSON.parse(process.argv[4]));
    } else {
      ret = await task(JSON.parse(process.argv[4]));
    }
    if (ret !== undefined) {
      airplane.setOutput(ret);
    }
  } catch (err) {
    console.error(err);
    // Print the error's message directly when possible. Otherwise, it includes the
    // error's name (e.g. "RunTerminationError: ...").
    const message = err instanceof Error ? err.message : String(err)
    console.log(`airplane_output_set:error ${JSON.stringify(message)}`);
    process.exit(1);
  }
}

main();
