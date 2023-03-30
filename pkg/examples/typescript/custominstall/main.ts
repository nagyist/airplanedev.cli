// Linked to https://app.airplane.dev/t/typescript_externals [do not edit this line]

import airplane from "airplane";
import * as pg from "pg";
import * as pgFormat from "pg-format";
// Prettier is a dev dependency only install because of the
// custom install.
import * as prettier from "prettier";

type Params = {
  id: string;
};

export default async function (params: Params) {
  airplane.appendOutput(params.id);

  airplane.appendOutput(Object.keys(airplane));
  airplane.appendOutput(Object.keys(pg));
  airplane.appendOutput(Object.keys(pgFormat));
  airplane.appendOutput(Object.keys(prettier));
}
