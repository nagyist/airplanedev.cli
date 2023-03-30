import airplane from "airplane";
import { name as pkg1name } from "pkg1";

export default airplane.view({ slug: "my_view", name: "My view" }, () => {
  return <div>{pkg1name}</div>;
});

console.log(pkg1name)
