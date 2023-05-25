import airplane from "airplane";
import { Title } from "@airplane/views";

const obj = {}

export const t1 = airplane.view(
  {
    ...obj,
    slug: "spread",
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);

// This view does not have a slug, so we can't add a test for it. However, its
// existence helps test whether the `good` view below can be updated when it exists
// in a file with a view like this.
export const t2 = airplane.view(
  obj,
  () => {
    return <Title>Hello World!</Title>;
  }
);

export const t3 = airplane.view(
  {
    slug: "computed",
    parameters: process.env.AIRPLANE_ENV_SLUG === "prod" ? {
      confirm: "boolean"
    } : {}
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);

const name = "parameters"
export const t4 = airplane.view(
  {
    [name]: {},
    slug: "key",
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);

export const t5 = airplane.view(
  {
    slug: "template",
    name: `Computed ${name}`
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);

export const t6 = airplane.view(
  {
    slug: "tagged_template",
    name: sql`Computed ${name}`
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);

// An example view that can be updated that is contained within a file
// with views that cannot be updated.
export const good = airplane.view(
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
  () => {
    return <Title>Hello World!</Title>;
  }
);
