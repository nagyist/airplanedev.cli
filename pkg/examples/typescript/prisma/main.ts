// Linked to https://app.airplane.dev/t/typescript_externals [do not edit this line]

import airplane from "airplane";
import { PrismaClient } from "@prisma/client";
type Params = {
  id: string;
};

export default async function (params: Params) {
  airplane.appendOutput(params.id);

  airplane.appendOutput(Object.keys(airplane));
  const prisma = new PrismaClient()
}
