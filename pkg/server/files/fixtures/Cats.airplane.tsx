import { Callout, Heading, Link, Stack, Text, Table } from "@airplane/views";
import airplane from "airplane";
const Demo = () => {
  return (
    <Stack spacing="lg">
      <Table
        title="Cats"
        data={[
          { name: "Xiaohuang", breed: "Abyssinian" },
          { name: "Peaches", breed: "Birman" },
          { name: "Baosky", breed: "British Shorthair" },
        ]}
      />
    </Stack>
  );
};
export default airplane.view(
  {
    slug: "demo",
    name: "Demo",
  },
  Demo
);
