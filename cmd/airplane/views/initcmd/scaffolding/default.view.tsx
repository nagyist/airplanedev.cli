import { Table, Column, Title, Text } from "@airplane/views";

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
    Header: "Element",
    accessor: "element",
  },
  {
    Header: "Weight",
    accessor: "weight",
  },
];

// Put the main logic of the view here.
// TODO: update documentation link
// Views documentation: https://docs.airplane.dev/
const ExampleView = () => {
  return (
    <>
      <Title>Elements</Title>
      <Text>An example view that showcases elements and their weights.</Text>
      <Table columns={columns} data={data} />
    </>
  );
};

export default ExampleView;
