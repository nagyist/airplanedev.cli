import airplane from "airplane";

// Put the main logic of the view here.
// Views documentation: https://docs.airplane.dev/views/getting-started
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
