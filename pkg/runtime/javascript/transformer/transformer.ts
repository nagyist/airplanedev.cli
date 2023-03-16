import { parse, print } from "@airplane/recast";
import * as typescript from "@airplane/recast/parsers/typescript";
import { ASTNode, builders, namedTypes, visit } from "ast-types";
import type { CommentKind, ExpressionKind, PatternKind } from "ast-types/gen/kinds";
import { readFile, writeFile } from "node:fs/promises";
import { inspect } from "node:util";

export const transform = async (file: string, existingSlug: string, def: any) => {
  const buf = await readFile(file);
  const contents = buf.toString();
  const ast = parse(contents, {
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

  let result = print(ast).code;

  // Recast always uses spaces for indentation even if the source file uses tabs.
  // This is an upstream bug that we could fix upstream, but for now we use a workaround.
  const originalIndentation = getIndentation(contents);
  if (originalIndentation === "tabs" && getIndentation(result) !== originalIndentation) {
    // Replace all indentation with tabs.
    result = result.replace(/^\t*( +)/gm, (_, match: string) => {
      // Since this file uses spaces, Recast will fallback to the default tabWidth (4). Therefore,
      // we know that 4 spaces is always equivalent to 1 tab.
      return "\t".repeat(match.length / 4);
    });
    if (getIndentation(result) !== originalIndentation) {
      console.error("Failed to re-format indentation to be consistent.");
    }
  }

  await writeFile(file, result);
};

const buildTaskConfig = (
  input: namedTypes.ObjectExpression,
  def: any
): namedTypes.ObjectExpression => {
  const output = builders.objectExpression([]);

  {
    output.properties.push(buildObjectProperty("slug", builders.stringLiteral(def.slug)));
  }

  if (def.name) {
    output.properties.push(buildObjectProperty("name", builders.stringLiteral(def.name)));
  }

  if (def.description) {
    output.properties.push(
      buildObjectProperty("description", builders.stringLiteral(def.description))
    );
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
            buildObjectProperty("default", buildParamValue(p.default, p.type))
          );
        }
        if (p.regex) {
          parameter.properties.push(buildObjectProperty("regex", builders.stringLiteral(p.regex)));
        }
        if (p.options) {
          parameter.properties.push(
            buildObjectProperty(
              "options",
              builders.arrayExpression(
                p.options.map((option) => {
                  const value = buildParamValue(option.value, p.type);
                  if (option.label) {
                    return builders.objectExpression([
                      buildObjectProperty("label", builders.stringLiteral(option.label)),
                      buildObjectProperty("value", value),
                    ]);
                  }
                  return value;
                })
              )
            )
          );
        }
        return buildObjectProperty(p.slug, parameter);
      })
    );
    output.properties.push(buildObjectProperty("parameters", parameters));
  }

  if (def.runtime && def.runtime !== "standard") {
    output.properties.push(buildObjectProperty("runtime", builders.stringLiteral(def.runtime)));
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
    output.properties.push(buildObjectProperty("timeout", builders.numericLiteral(def.timeout)));
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
    output.properties.push(
      buildObjectProperty("requireRequests", builders.booleanLiteral(def.requireRequests))
    );
  }

  if (def.allowSelfApprovals === false) {
    output.properties.push(
      buildObjectProperty("allowSelfApprovals", builders.booleanLiteral(def.allowSelfApprovals))
    );
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
    Object.entries(paramValues).map(([param, paramValue]) =>
      buildObjectProperty(param, buildParamValue(paramValue))
    )
  );
};

const buildParamValue = (paramValue: any, type?: string): ExpressionKind => {
  // Certain parameter kinds are serialized in specific ways.
  if (type === "datetime" && typeof paramValue === "string") {
    // Rewrite datetimes using the Date object.
    return builders.newExpression(builders.identifier("Date"), [
      builders.stringLiteral(paramValue),
    ]);
  }
  if (
    type === "configvar" &&
    typeof paramValue === "object" &&
    typeof paramValue["config"] === "string"
  ) {
    // Rewrite the legacy config var format (used in YAML definitions).
    return builders.stringLiteral(paramValue["config"]);
  }

  return buildJSON(paramValue);
};

/** Serialize a value as a JSON object. */
const buildJSON = (value: any): ExpressionKind => {
  if (value == null) {
    return builders.nullLiteral();
  }
  if (typeof value === "string") {
    return builders.stringLiteral(value);
  }
  if (typeof value === "number") {
    return builders.numericLiteral(value);
  }
  if (typeof value === "boolean") {
    return builders.booleanLiteral(value);
  }
  if (Array.isArray(value)) {
    return builders.arrayExpression(value.map(buildJSON));
  }
  if (typeof value === "object") {
    return builders.objectExpression(
      Object.keys(value).map((key) => buildObjectProperty(key, buildJSON(value[key])))
    );
  }
  throw new Error(`Unable to serialize value as JSON: ${value}`);
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

/**
 * Inspects a file's contents and determines if it consistently uses tabs or spaces.
 *
 * If it uses a mix of indentation (or none), it returns undefined.
 */
const getIndentation = (contents: string): "tabs" | "spaces" | undefined => {
  if (!contents) {
    return undefined;
  }
  const regex = /^[ \t]*/gm;
  let tabLines = 0;
  let spaceLines = 0;
  let m: RegExpExecArray | null;
  while ((m = regex.exec(contents)) !== null) {
    // This is necessary to avoid infinite loops with zero-width matches
    if (m.index === regex.lastIndex) {
      regex.lastIndex++;
    }

    // The result can be accessed through the `m`-variable.
    m.forEach((match) => {
      if (match.includes(" ")) {
        spaceLines++;
        if (tabLines > 0) {
          // There is a mixture of indentation. Return early.
          return undefined;
        }
      }
      if (match.includes("\t")) {
        tabLines++;
        if (spaceLines > 0) {
          // There is a mixture of indentation. Return early.
          return undefined;
        }
      }
    });
  }

  if (spaceLines && !tabLines) {
    return "spaces";
  }
  if (!spaceLines && tabLines) {
    return "tabs";
  }
  return undefined;
};
