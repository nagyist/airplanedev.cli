import { Column, Stack, Table, Text, Title } from "@airplane/views";
import { AcademicCapIcon } from "@airplane/views/icons";
import airplane from "airplane";

import { helper } from "./helper";

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
      <Title>Elements</Title>
      <Text>An example view that showcases elements and their weights.</Text>
      <Text>{helper()}</Text>
      <Table title="Elements Table" columns={columns} data={data} />
      <AcademicCapIcon />
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
