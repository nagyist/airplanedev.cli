import "reflect-metadata";
import airplane from "airplane";

function logType(target: any, key: string) {
  var t = Reflect.getMetadata("design:type", target, key);
  console.log(`${key} type: ${t.name}`);
}

class Demo {
  @logType // apply property decorator
  public attr1: string = "";
}

export default airplane.task(
  {
    slug: "tas",
    name: "task",
  },
  () => {
    new Demo();
  }
);
