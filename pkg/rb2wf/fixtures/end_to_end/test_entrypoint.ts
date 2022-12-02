import airplane from "airplane";

// Converted from runbook test_runbook_slug (id: testID)
// via `airplane task init --from-runbook test_runbook_slug`
export default airplane.workflow(
  {
    slug: "test_runbook_slug",
    name: "testRunbook (workflow)",
    parameters: {
      a_boolean_slug: {
        name: "A boolean",
        slug: "a_boolean_slug",
        type: "boolean",
      },
      a_date_slug: {
        default: "2022-11-18",
        name: "A date",
        slug: "a_date_slug",
        type: "date",
      },
      an_integer_slug: {
        default: 3,
        name: "An integer",
        slug: "an_integer_slug",
        type: "integer",
      },
      test_param_slug: {
        default: "512",
        name: "Test param",
        slug: "test_param_slug",
        type: "shorttext",
      },
    },
    resources: ["db_slug", "email_slug", "rest_slug"],
    envVars: { dbdsn: { config: "dbdsn" } },
  },
  async (params) => {
    // Set globals; see https://docs.airplane.dev/runbooks/javascript-templates#globals-reference
    // for more details.

    // Sessions don't exist in workflows, so the associated globals are unset. If you're using them
    // in your code, you'll want to remove them.
    const session = {
      id: "unset",
      url: "unset",
      name: "unset",
      creator: {
        id: "unset",
        email: "unset",
        name: "unset",
      },
    };

    // Blocks don't exist in tasks, so these are unset. If you're using them in your code,
    // you'll want to remove them.
    const block = {
      id: "unset",
      slug: "unset",
    };

    // This is no longer a runbook, so the runbook globals are unset. If you're using them
    // in your code, you'll want to remove them.
    const runbook = {
      id: "unset",
      url: "unset",
      name: "unset",
    };

    const env = {
      id: "unset",
      slug: "unset",
      name: "unset",
      is_default: false,
    };
    // Get configs from the environment.
    const configs = { dbdsn: process.env["dbdsn"] };

    const task_block_slug = await airplane.execute<any>("test_task_slug", {
      count: params.an_integer_slug,
    });

    await airplane.display.markdown(
      `This is some content with a ${params.an_integer_slug}`
    );

    const sql_block_slug = await airplane.sql.query<any>(
      "db_slug",
      "SELECT count(*) from users limit :user_count;",
      {
        args: { user_count: params.an_integer_slug },
      }
    );

    let rest_block_slug: any;

    if ("hello" === params.test_param_slug) {
      rest_block_slug = await airplane.rest.request<any>(
        "rest_slug",
        "GET",
        "/heathz",
        { headers: { header1: "header2" }, urlParams: { test1: "test2" } }
      );
    } else {
      console.log(
        "Skipping over 'rest_block_slug' because startCondition is false"
      );
    }

    await airplane.email.message(
      "email_slug",
      { email: "yolken@airplane.dev", name: "BHY" },
      [{ email: "bob@example.com", name: "Bob" }],
      { message: "This is a message!", subject: "Hello" }
    );

    await airplane.slack.message("notif-deploys-test", "Hello!");

    // The conversion of this block type hasn't been implemented yet:
    //
    // {
    //  "id": "formBlockID",
    //  "kind": "form",
    //  "kindConfig": {
    //   "form": {
    //    "parameters": {
    //     "parameters": [
    //      {
    //       "name": "Name",
    //       "slug": "name",
    //       "type": "string",
    //       "desc": "",
    //       "component": "",
    //       "default": {
    //        "__airplaneType": "template",
    //        "raw": "Hello"
    //       },
    //       "constraints": {
    //        "optional": false,
    //        "regex": ""
    //       }
    //      }
    //     ]
    //    },
    //    "paramValues": null
    //   }
    //  },
    //  "startCondition": "",
    //  "slug": "form_block_slug"
    // }
    let form_block_slug: any;
  }
);
