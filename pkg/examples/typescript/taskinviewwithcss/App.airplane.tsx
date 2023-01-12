import airplane from "airplane";
// Import a local CSS file.
import "./styles.css";
// Import a css file from a dependency.
import "react-datepicker/dist/react-datepicker.css";

// Put the main logic of the view here.
// Views documentation: https://docs.airplane.dev/views/getting-started
const ExampleView = () => {
  return <div className="some-style" />;
};

export default airplane.view(
  {
    name: "My View",
    slug: "my_view",
    description: "my description",
  },
  ExampleView
);

export const tt = airplane.task(
  {
    slug: "tas",
    name: "task",
  },
  () => {
    return "running:in_view";
  }
);
