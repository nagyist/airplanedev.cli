// After changing this file, run `yarn build` to bundle into a JS file. Or
// use `yarn build:watch` to rebuild after every change.

// To learn more about how to use recast, see their README for docs:
// - https://github.com/benjamn/recast (parsing/printing logic)
// - https://github.com/benjamn/ast-types (core "types" like "ObjectExpression")
//
// You can also visually explore the parsed AST:
// - https://astexplorer.net/#/gist/39078d85ea26b8553d533aeb7c235c9f/4858d0d7ccff7b0c3bacc9ea2aff8ff395601518
//
// The recast pretty-printer has some outdated styling opinions. We forked recast so we can
// tweak these behaviors.
//
// 1. If an object parameter has multiple lines, a newline will be inserted above and below it.
//    See: https://github.com/benjamn/recast/issues/228
//    Fix: https://github.com/airplanedev/recast/pull/1
import { parse, print } from "@airplane/recast";
import * as typescript from "@airplane/recast/parsers/typescript";
import { namedTypes, ASTNode, visit, builders } from "ast-types";
import type { CommentKind, ExpressionKind, PatternKind } from "ast-types/gen/kinds";
import { writeFile, readFile } from "node:fs/promises";
import { inspect } from "node:util";

export const transform = async (file: string, existingSlug: string, def: any) => {
  const buf = await readFile(file);
  const ast = parse(buf.toString(), {
    parser: typescript,
    // When printing string literals, prefer the quote that will generate the shortest literal.
    quote: "auto",
  }) as ASTNode;
  visit(ast, {
    // `airplane.task(...)` is a CallExpression
    visitCallExpression(path) {
      const { node } = path;

      // Check if this is an `airplane.task/workflow/view` call.
      const airplaneExpression = isAirplaneExpression(node);
      if (!airplaneExpression) {
        return this.traverse(path);
      }

      if (airplaneExpression === "airplane.view") {
        // TODO: support views!
        return this.traverse(path);
      }

      const arg1 = node.arguments[0];
      if (namedTypes.ObjectExpression.check(arg1)) {
        // Continue...
      } else {
        throw new Error(
          `Cannot inspect task options due to unsupported syntax "${arg1.type}"${printLOC(
            arg1.loc
          )}`
        );
      }

      // There may be multiple tasks in this file. Confirm this task's slug matches
      // the one we're editing.
      const slug = getStringValue(arg1, "slug");
      if (slug !== existingSlug) {
        return this.traverse(path);
      }

      const newNode = buildTaskConfig(arg1, def);
      node.arguments[0] = newNode;

      // Continue traversing.
      return this.traverse(path);
    },
  });

  const result = print(ast);
  await writeFile(file, result.code);
};

const buildTaskConfig = (
  input: namedTypes.ObjectExpression,
  def: any
): namedTypes.ObjectExpression => {
  const output = builders.objectExpression([]);

  {
    const value = builders.stringLiteral(def.slug);
    value.comments = getComments(input, "slug");
    output.properties.push(buildObjectProperty("slug", value));
  }

  if (def.name) {
    const value = builders.stringLiteral(def.name);
    value.comments = getComments(input, "name");
    output.properties.push(buildObjectProperty("name", value));
  }

  if (def.description) {
    const value = builders.stringLiteral(def.description);
    value.comments = getComments(input, "description");
    output.properties.push(buildObjectProperty("description", value));
  }

  if (def.parameters && def.parameters.length > 0) {
    const parameters = builders.objectExpression(
      def.parameters.map((p) => {
        const parameter = builders.objectExpression([]);
        if (p.name) {
          parameter.properties.push(buildObjectProperty("name", builders.stringLiteral(p.name)));
        }
        if (p.description) {
          parameter.properties.push(
            buildObjectProperty("description", builders.stringLiteral(p.description))
          );
        }
        if (p.type) {
          parameter.properties.push(buildObjectProperty("type", builders.stringLiteral(p.type)));
        }
        if (p.required === false) {
          parameter.properties.push(
            buildObjectProperty("required", builders.booleanLiteral(p.required))
          );
        }
        if (p.default != null) {
          parameter.properties.push(
            buildObjectProperty("default", builders.booleanLiteral(p.default))
          );
        }
        return buildObjectProperty(p.slug, parameter);
      })
    );
    output.properties.push(buildObjectProperty("parameters", parameters));
  }

  if (def.runtime && def.runtime !== "standard") {
    const value = builders.stringLiteral(def.runtime);
    value.comments = getComments(input, "runtime");
    output.properties.push(buildObjectProperty("runtime", value));
  }

  if (def.resources) {
    if (Array.isArray(def.resources)) {
      if (def.resources.length > 0) {
        const value = builders.arrayExpression(def.resources.map(builders.stringLiteral));
        output.properties.push(buildObjectProperty("resources", value));
      }
    } else if (Object.keys(def.resources).length > 0) {
      const value = builders.objectExpression([]);
      for (const [resourceAlias, resourceSlug] of Object.entries<string>(def.resources)) {
        value.properties.push(
          buildObjectProperty(resourceAlias, builders.stringLiteral(resourceSlug))
        );
      }
      output.properties.push(buildObjectProperty("resources", value));
    }
  }

  if (def.node && def.node.envVars && Object.keys(def.node.envVars).length > 0) {
    const value = builders.objectExpression([]);
    for (const envVar in def.node.envVars) {
      const envVarValue = def.node.envVars[envVar];
      var propertyValue: ExpressionKind;
      if (typeof envVarValue === "string") {
        propertyValue = builders.stringLiteral(envVarValue);
      } else if (envVarValue["config"]) {
        propertyValue = builders.objectExpression([
          buildObjectProperty("config", builders.stringLiteral(envVarValue["config"])),
        ]);
      } else {
        propertyValue = builders.stringLiteral(envVarValue["value"] ?? "");
      }
      value.properties.push(buildObjectProperty(envVar, propertyValue));
    }
    output.properties.push(buildObjectProperty("envVars", value));
  }

  const defaultTimeout = def.runtime === "workflow" ? 0 : 3600;
  if (def.timeout && def.timeout !== defaultTimeout) {
    const value = builders.numericLiteral(def.timeout);
    value.comments = getComments(input, "timeout");
    output.properties.push(buildObjectProperty("timeout", value));
  }

  if (def.constraints && Object.keys(def.constraints).length > 0) {
    const value = builders.objectExpression([]);
    for (const [constraint, constraintValue] of Object.entries<string>(def.constraints)) {
      value.properties.push(
        buildObjectProperty(constraint, builders.stringLiteral(constraintValue))
      );
    }
    output.properties.push(buildObjectProperty("constraints", value));
  }

  if (def.requireRequests) {
    const value = builders.booleanLiteral(def.requireRequests);
    value.comments = getComments(input, "requireRequests");
    output.properties.push(buildObjectProperty("requireRequests", value));
  }

  if (def.allowSelfApprovals === false) {
    const value = builders.booleanLiteral(def.allowSelfApprovals);
    value.comments = getComments(input, "allowSelfApprovals");
    output.properties.push(buildObjectProperty("allowSelfApprovals", value));
  }

  if (def.schedules && Object.keys(def.schedules).length > 0) {
    const schedules = builders.objectExpression([]);
    for (const [alias, s] of Object.entries<any>(def.schedules)) {
      const schedule = builders.objectExpression([]);
      if (s.name) {
        schedule.properties.push(buildObjectProperty("name", builders.stringLiteral(s.name)));
      }
      if (s.description) {
        schedule.properties.push(
          buildObjectProperty("description", builders.stringLiteral(s.description))
        );
      }
      if (s.cron) {
        schedule.properties.push(buildObjectProperty("cron", builders.stringLiteral(s.cron)));
      }
      if (s.paramValues && Object.keys(s.paramValues).length > 0) {
        schedule.properties.push(
          buildObjectProperty("paramValues", buildParamValues(s.paramValues))
        );
      }
      schedules.properties.push(buildObjectProperty(alias, schedule));
    }
    output.properties.push(buildObjectProperty("schedules", schedules));
  }

  return output;
};

/**
 * Shorthand for `builders.objectProperty` which also converting key values to identifiers
 * if they are valid (else will use a string literal).
 */
const buildObjectProperty = (key: string, value: any) => {
  // Checks if this key is a valid JavaScript identifier. If not, we have to wrap it
  // with string quotes.
  const keyExpression = /^[a-zA-Z_$][0-9a-zA-Z_$]*$/.test(key)
    ? builders.identifier(key)
    : builders.stringLiteral(key);
  return builders.objectProperty(keyExpression, value);
};

const buildParamValues = (paramValues: any): namedTypes.ObjectExpression => {
  return builders.objectExpression(
    Object.entries(paramValues).map(([param, paramValue]) => {
      if (typeof paramValue === "string") {
        return buildObjectProperty(param, builders.stringLiteral(paramValue));
      }
      if (typeof paramValue === "number") {
        return buildObjectProperty(param, builders.numericLiteral(paramValue));
      }
      if (typeof paramValue === "boolean") {
        return buildObjectProperty(param, builders.booleanLiteral(paramValue));
      }
      throw new Error(`Unhandled parameter value type: ${paramValue}`);
    })
  );
};

const airplaneExpressions = ["airplane.task", "airplane.workflow", "airplane.view"] as const;
type AirplaneExpression = (typeof airplaneExpressions)[number];

/**
 * Checks if this node is an `airplane.[task|view|...]` expression, otherwise returning `undefined`.
 */
const isAirplaneExpression = (node: namedTypes.CallExpression): AirplaneExpression | undefined => {
  const { callee } = node;
  if (!namedTypes.MemberExpression.check(callee)) {
    return undefined;
  }
  const name = getMemberExpressionName(callee);

  return name && airplaneExpressions.includes(name as any)
    ? (name as AirplaneExpression)
    : undefined;
};

const getMemberExpressionName = (e: namedTypes.MemberExpression): string | undefined => {
  const { object, property } = e;
  if (namedTypes.Identifier.check(object) && namedTypes.Identifier.check(property)) {
    return object.name + "." + property.name;
  }
  return undefined;
};

/**
 * Returns the value of the "slug" field. Returns an empty string if not set.
 */
const getStringValue = (e: namedTypes.ObjectExpression, fieldName: string): string => {
  const value = getPropertyValue(e, fieldName);
  if (!value) {
    return "";
  }

  if (namedTypes.StringLiteral.check(value)) {
    return value.value;
  } else {
    throw new Error(
      `Cannot get slug due to unsupported value syntax "${value.type}"${printLOC(value.loc)}`
    );
  }
};

const getComments = (
  e: namedTypes.ObjectExpression,
  fieldName: string
): CommentKind[] | null | undefined => {
  const value = getPropertyValue(e, fieldName);
  if (!value) {
    return undefined;
  }

  if (namedTypes.StringLiteral.check(value) || namedTypes.TemplateLiteral.check(value)) {
    console.log(`Found comments for ${fieldName}: ${inspect(value.comments)}`);
    return value.comments;
  } else {
    // There are too many cases to handle here (since `value` can be any expression), so we can't `assertNever(value)`.
    throw new Error(
      `Cannot edit field "${fieldName}" due to unsupported value syntax "${value.type}"${printLOC(
        value.loc
      )}`
    );
  }
};

const getPropertyValue = (
  e: namedTypes.ObjectExpression,
  fieldName: string
): ExpressionKind | PatternKind | undefined => {
  for (const [i, property] of e.properties.entries()) {
    if (namedTypes.ObjectProperty.check(property)) {
      // Continue...
    } else if (
      namedTypes.SpreadProperty.check(property) ||
      namedTypes.ObjectMethod.check(property) ||
      namedTypes.Property.check(property) ||
      namedTypes.SpreadElement.check(property)
    ) {
      // Ignore...
      continue;
    } else {
      return assertNever(property);
    }

    const { key } = property;
    var keyName: string;
    if (namedTypes.Identifier.check(key)) {
      keyName = key.name;
    } else if (namedTypes.StringLiteral.check(key)) {
      keyName = key.value;
    } else {
      // There are too many cases to handle here (since `key` can be any expression), so we can't `assertNever(key)`.
      throw new Error(
        `Cannot inspect field "${fieldName}" due to unsupported syntax "${key.type}"${printLOC(
          key.loc
        )}`
      );
    }

    console.log(`Found field ${keyName}`);
    if (keyName !== fieldName) {
      // This is not the property we want to edit.
      continue;
    }

    return property.value;
  }

  return undefined;
};

const printLOC = (loc: namedTypes.SourceLocation | null | undefined): string => {
  if (!loc) {
    return "";
  }
  return ` at ${loc.start?.line}:${loc.start?.column}...${loc.end?.line}:${loc.end?.column}`;
};

/**
 * Use assertNever at the end of exhaustive checks of discriminated unions.
 * TypeScript will error if it becomes non-exhaustive.
 */
const assertNever = (value: never): never => {
  const desc = value && "type" in value ? (value as any).type : JSON.stringify(value);
  throw new Error(`Unhandled syntax: ${desc}`);
};
