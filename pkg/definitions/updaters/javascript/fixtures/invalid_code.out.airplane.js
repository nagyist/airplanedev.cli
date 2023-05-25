// This file includes a few intentional "bugs" to ensure the update gracefully
// handles them.

// Bug: `airplane` is not imported.

// Bug: unknown function reference
ipsum()

export default airplane.task(
  {
    slug: "my_task",
    description: "Added a description!"
  },
  async () => {
    // Bug: unknown variable reference (foo).
    console.log(foo)
  }
);
