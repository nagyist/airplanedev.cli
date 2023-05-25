import airplane from "airplane";
import { Title } from "@airplane/views";

export const myEntity2 = airplane.task(
  {
    // This slug is the same as the View. It should not get updated.
    slug: "my_entity_2"
  },
  () => {
    return <Title>Hello World!</Title>;
  }
)

export const myEntity = airplane.view(
  {
    slug: "my_entity"
  },
  () => {
    return <Title>Hello World!</Title>;
  }
)

export default airplane.view(
  {
    slug: "my_entity_2",
    name: "My entity"
  },
  () => {
    return <Title>Hello World!</Title>;
  }
);
