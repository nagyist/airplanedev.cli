import airplane from "airplane";

function LogMethod(
  target: any,
  propertyKey: string | symbol,
  descriptor: PropertyDescriptor
) {
  console.log("Decorated")
}

class Demo {
  @LogMethod
  public foo() {
    // do nothing
  }
}

export default airplane.task(
  {
    slug: "tas",
    name: "task",
  },
  () => {
    new Demo().foo();
  }
);
