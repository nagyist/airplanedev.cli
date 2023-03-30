import React from "react";
import airplane from "airplane";

function App() {
  return <div>Hello world</div>;
}

export default airplane.view(
  {
    name: "App2",
    slug: "app2",
    description: "A simple view2",
  },
  App
);
