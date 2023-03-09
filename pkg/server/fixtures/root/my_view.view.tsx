import { Column, Stack, Table, Text, Heading } from "@airplane/views";
import airplane from "airplane";

// Example data
type Row = {
  element: string;
  weight: string;
};

const data: Row[] = [
  {
    element: "Hydrogen",
    weight: "1.008",
  },
  {
    element: "Helium",
    weight: "4.0026",
  },
];

const columns: Column[] = [
  {
    label: "Element",
    accessor: "element",
  },
  {
    label: "Weight",
    accessor: "weight",
  },
];

// Put the main logic of the view here.
// Views documentation: https://docs.airplane.dev/views/getting-started

const ExampleView = () => {
  return (
    <Stack>
      <Heading>Elements</Heading>
      <Text>An example view that showcases elements and their weights.</Text>
      <Table title="Elements Table" columns={columns} data={data} />
    </Stack>
  );
};

export default airplane.view(
  {
    name: "My View",
    slug: "my_view",
    description: "my description",
  },
  ExampleView
);
