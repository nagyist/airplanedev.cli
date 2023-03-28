import { Heading, Stack, Table } from "@airplane/views";

const {{.ViewName}} = () => {
  return (
    <Stack>
      <Heading>Users</Heading>
      <Table
        title="Users"
        data={[
          { name: "Frances Hernandez", role: "Engineer" },
          { name: "Charlotte Morris", role: "Sales" },
          { name: "Ju Hsiao", role: "Support" },
        ]}
      />
    </Stack>
  );
};

export default {{.ViewName}};
