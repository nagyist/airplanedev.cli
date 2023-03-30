import React from "react";
import airplane from "airplane";

function App() {
  return <div>Hello world</div>;
}

export default airplane.view(
  {
    name: "App",
    slug: "app",
    description: "A simple view",
  },
  App
);
