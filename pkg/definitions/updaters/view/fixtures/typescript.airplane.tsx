import {Title} from "@airplane/views";
import airplane from "airplane";

export type HelloWorldProps = {
  name: string
}

const HelloWorld = ({ name }: HelloWorldProps) => {
  return <Title>Hello, {name}!</Title>;
}

export default airplane.view(
  {
    slug: "my_view"
  },
  () => {
    return <HelloWorld name="Colin" />;
  }
);
