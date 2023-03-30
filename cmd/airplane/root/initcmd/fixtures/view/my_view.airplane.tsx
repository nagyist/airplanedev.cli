import { Callout, Heading, Link, Stack, Text } from "@airplane/views";
import airplane from "airplane";

const MyView = () => {
  return (
    <Stack spacing="lg">
      <Stack spacing={0}>
        <Heading>👋 Hello, world!</Heading>
        <Text>Views make it easy to build UIs in Airplane.</Text>
      </Stack>
      <Stack spacing={0}>
        <Heading level={3}>Learn more</Heading>
        <Stack direction="row">
          <Callout variant="neutral" title="Overview" width="1/3">
            {"Check out what you can build with views. "}
            <Link href="https://docs.airplane.dev/views/overview" size="sm">
              Read the docs.
            </Link>
          </Callout>
          <Callout variant="neutral" title="Build your first view" width="1/3">
            {"Walk through building a simple view in 15 minutes. "}
            <Link href="https://docs.airplane.dev/getting-started/views" size="sm">
              Read the docs.
            </Link>
          </Callout>
          <Callout variant="neutral" title="Component library" width="1/3">
            {"Browse the Views component library. "}
            <Link href="https://docs.airplane.dev/views/components" size="sm">
              Read the docs.
            </Link>
          </Callout>
        </Stack>
      </Stack>
    </Stack>
  );
};

export default airplane.view(
  {
    slug: "my_view",
    name: "My view",
  },
  MyView
);
