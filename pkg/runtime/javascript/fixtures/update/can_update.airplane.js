import airplane from "airplane";

const obj = {}

export const t1 = airplane.task(
  {
    ...obj,
    slug: "spread",
  },
  async () => {
    return []
  },
);

// This task does not have a slug, so we can't add a test for it. However, its
// existence helps test whether the `good` task below can be updated when it exists
// in a file with a task like this.
export const t2 = airplane.task(
  obj,
  async () => {
    return []
  },
);

export const t3 = airplane.task(
  {
    slug: "computed",
    parameters: process.env.AIRPLANE_ENV_SLUG === "prod" ? {
      confirm: "boolean"
    } : {}
  },
  async () => {
    return []
  },
);

const name = "parameters"
export const t4 = airplane.task(
  {
    [name]: {},
    slug: "key",
  },
  async () => {
    return []
  },
);

export const t5 = airplane.task(
  {
    slug: "template",
    name: `Computed ${name}`
  },
  async () => {
    return []
  },
);

export const t6 = airplane.task(
  {
    slug: "tagged_template",
    name: sql`Computed ${name}`
  },
  async () => {
    return []
  },
);

// An example task that can be updated that is contained within a file
// with tasks that cannot be updated.
export const good = airplane.task(
  {
    slug: "good",
    // Various value types that should not be flagged as computed:
    a: undefined,
    b: null,
    c: 123,
    d: 12.34,
    e: "hello",
    f: 'world',
    // The following two template literals are not computed.
    g: `test`,
    h: sql`test`,
    i: ["hello"],
    j: {jj: "world"},
    k: true,
    l: false,
  },
  async () => {
    return []
  },
);
