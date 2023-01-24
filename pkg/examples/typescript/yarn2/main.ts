// Linked to https://app.airplane.dev/t/typescript_yarn [do not edit this line]
import { execSync } from "node:child_process";
import airplane from 'airplane'

type Params = {
  id: string
}

export default async function(params: Params) {
  airplane.appendOutput(execSync("yarn --version").toString());
}
