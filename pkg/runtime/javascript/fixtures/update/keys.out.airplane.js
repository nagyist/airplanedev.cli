import airplane from "airplane";

export default airplane.task(
  {
    slug: "my_task",
    constraints: {
      a_valid_identifier: "...",
      "an invalid identifier": "...",
      "both'\"'\"": "'\"'\"",
      'double"': '"',
      "single'": "'"
    }
  },
  async () => {
    return []
  },
);
