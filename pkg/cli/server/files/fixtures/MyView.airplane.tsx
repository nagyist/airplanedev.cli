import {
  Column,
  Stack,
  Text,
  Heading,
  useComponentState,
} from "@airplane/views";
import airplane from "airplane";

// Put the main logic of the view here.
// Views documentation: https://docs.airplane.dev/views/getting-started
const ExampleView = () => {
  const customersState = useComponentState();
  const selectedCustomer = customersState.selectedRow;

  return (
    <Stack>
      <Heading>Customer overview</Heading>
      <Text>An example view that showcases customers and users.</Text>
    </Stack>
  );
};
