import {
  Column,
  Stack,
  Table,
  Text,
  Heading,
  useComponentState,
} from "@airplane/views";

// Put the main logic of the view here.
// Views documentation: https://docs.airplane.dev/views/getting-started
const ExampleView = () => {
  const customersState = useComponentState();
  const selectedCustomer = customersState.selectedRow;

  return (
    <Stack>
      <Heading>Customer overview</Heading>
      <Text>An example view that showcases customers and users.</Text>
      <Table
        id={customersState.id}
        title="Customers"
        columns={customersCols}
        data={customersData}
        rowSelection="single"
      />
      {selectedCustomer && (
        <Table
          title={`Users for ${selectedCustomer.name}`}
          data={usersData.filter((u) => u.customer_id == selectedCustomer.id)}
          columns={usersCols}
          hiddenColumns={["customer_id"]}
        />
      )}
    </Stack>
  );
};

// Example data: replace with your own data or use an Airplane task
// Data fetching documentation: https://docs.airplane.dev/views/data-fetching
const customersData = [
  {
    id: "0",
    name: "Future Golf Partners",
    country: "Brazil",
  },
  {
    id: "1",
    name: "Blue Sky Corp",
    country: "France",
  },
];

const usersData = [
  {
    id: "1",
    customer_id: "0",
    name: "Frances Hernandez",
    role: "Engineer",
    email: "frances.hernandez@futuregolfpartners.com",
  },
  {
    id: "2",
    customer_id: "0",
    name: "Melissa Rodriguez",
    role: "Engineer",
    email: "melissa.rodriguez@futuregolfpartners.com",
  },
  {
    id: "3",
    customer_id: "1",
    name: "Roy Hernandez",
    role: "Sales",
    email: "roy.hernandez.1@blueskycorp.us",
  },
];
// End of example data

const customersCols: Column[] = [
  {
    label: "Customer ID",
    accessor: "id",
  },
  {
    label: "Name",
    accessor: "name",
  },
  {
    label: "Country",
    accessor: "country",
  },
];

const usersCols: Column[] = [
  {
    label: "User ID",
    accessor: "id",
  },
  {
    label: "Name",
    accessor: "name",
  },
  {
    label: "Role",
    accessor: "role",
  },
  {
    label: "Email",
    accessor: "email",
  },
];

export default ExampleView;
