// Linked to https://app.airplane.dev/t/typescript_externals [do not edit this line]

import airplane from "airplane";
// prettier is installed in the custom postinstall.
// @ts-ignore
import * as prettier from "prettier";
// @ts-ignore
import * as fs from 'fs';

type Params = {
  id: string;
};

export default async function (params: Params) {
  airplane.appendOutput(params.id);
  airplane.appendOutput(fs.readFileSync('./preinstall.txt','utf8'));
  airplane.appendOutput(Object.keys(airplane));
  airplane.appendOutput(Object.keys(prettier));
}
