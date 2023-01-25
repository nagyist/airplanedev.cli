import airplane from "airplane";

const ExampleView = () => {
  return <div>hi</div>;
};

export default airplane.view(
  {
    name: "My View",
    slug: "my_view",
  },
  ExampleView
);
