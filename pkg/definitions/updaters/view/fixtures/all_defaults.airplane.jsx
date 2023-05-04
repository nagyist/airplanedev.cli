import airplane from "airplane";
import { Title } from "@airplane/views";

export default airplane.view(
  {
    slug: "my_view"
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);
