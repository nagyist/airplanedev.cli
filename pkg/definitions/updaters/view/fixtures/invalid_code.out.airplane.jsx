// This file includes a few intentional "bugs" to ensure the update gracefully
// handles them.

// Bug: `airplane` is not imported.

// Bug: unknown function reference
ipsum()

export default airplane.view(
  {
    slug: "my_view",
    description: "Added a description!"
  },
  () => {
    // Bug: unknown variable reference (foo).
    return <Title>Hello {foo}!</Title>;
  }
);
