// This file includes a few intentional "bugs" to ensure the transformer gracefully
// handles them.

// Bug: `airplane` is not imported.

// Bug: unknown function reference
ipsum()

// Bug: Invalid export syntax (needs a name or "default").
export default airplane.task(
  {
    slug: "my_task"
  },
  async () => {
    // Bug: unknown variable reference (foo).
    console.log(foo)
  }
);
