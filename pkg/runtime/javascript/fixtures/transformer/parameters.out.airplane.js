import airplane from "airplane";

export default airplane.task(
  {
    slug: "my_task",
    parameters: {
      all: {
        name: "All fields",
        description: "My description",
        type: "shorttext",
        required: false,
        default: "My default",
        regex: "^.*$",
        options: [{
          label: "Thing 3",
          value: "Secret gremlin"
        }]
      },
      shorttext: {
        type: "shorttext",
        default: "Text"
      },
      longtext: {
        type: "longtext",
        default: "Longer text"
      },
      sql: {
        type: "sql",
        default: "SELECT 1"
      },
      boolean_true: {
        type: "boolean",
        default: true
      },
      boolean_false: {
        type: "boolean",
        default: false
      },
      upload: {
        type: "upload",
        default: "upl123"
      },
      integer: {
        type: "integer",
        default: 10
      },
      integer_zero: {
        type: "integer",
        default: 0
      },
      float: {
        type: "float",
        default: 3.14
      },
      float_zero: {
        type: "float",
        default: 0
      },
      date: {
        type: "date",
        default: "2006-01-02"
      },
      datetime: {
        type: "datetime",
        default: new Date("2006-01-02T15:04:05Z07:00")
      },
      configvar: {
        type: "configvar",
        default: "MY_CONFIG"
      },
      configvar_legacy: {
        type: "configvar",
        default: "MY_CONFIG"
      }
    }
  },
  async () => {
    return []
  },
);
