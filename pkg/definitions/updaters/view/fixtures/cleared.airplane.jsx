import airplane from "airplane";
import { Title } from "@airplane/views";

export default airplane.view(
  {
    slug: "my_view",
    name: "View name",
    description: "View description",
    envVars: {
      CONFIG: {
        config: "aws_access_key"
      },
      VALUE: {
        value: "Hello World!"
      }
    }
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);
