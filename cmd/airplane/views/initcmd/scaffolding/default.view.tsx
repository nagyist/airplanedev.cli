import { Callout, Heading, Link, Stack, Text } from "@airplane/views";

const {{.ViewName}} = () => {
  return (
    <Stack spacing="lg">
      <Stack spacing={0}>
        <Heading>👋 Hello, world!</Heading>
        <Text>Views make it easy to build UIs in Airplane.</Text>
      </Stack>
      <Stack spacing={0}>
        <Heading level={3}>Learn more</Heading>
        <Stack direction="row">
          <Callout variant="neutral" title="Build your first view" width="1/3">
            {"Walk through building a simple view in 15 minutes. "}
            <Link href="https://docs.airplane.dev/getting-started/views" size="sm">
              Read the docs.
            </Link>
          </Callout>
          <Callout variant="neutral" title="Templates" width="1/3">
            {"Work off of one of our many ready-made templates. "}
            <Link href="https://docs.airplane.dev/templates" size="sm">
              See our templates.
            </Link>
          </Callout>
          <Callout variant="neutral" title="Reference" width="1/3">
            {"Learn more about how to supercharge your views. "}
            <Link href="https://docs.airplane.dev" size="sm">
              Read the docs.
            </Link>
          </Callout>
        </Stack>
      </Stack>
    </Stack>
  );
};

export default {{.ViewName}};
