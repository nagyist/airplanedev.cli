import airplane from "airplane";
import { Title } from "@airplane/views";

export const myTask = airplane.task(
  {
    slug: "my_task",
    description: "Added a description!"
  },
  () => {
    return [];
  }
)

export default airplane.view(
  {
    slug: "my_view"
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);
