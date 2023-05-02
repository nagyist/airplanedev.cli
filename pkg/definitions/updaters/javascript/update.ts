import { parse, print } from "@airplane/recast";
import * as typescript from "@airplane/recast/parsers/typescript";
import { ASTNode, builders, namedTypes, visit } from "ast-types";
import type { ExpressionKind, PatternKind } from "ast-types/gen/kinds";
import { readFile, writeFile } from "node:fs/promises";

export const update = async (
  file: string,
  existingSlug: string,
  def: any,
  opts: { dryRun: boolean }
) => {
  const buf = await readFile(file);
  const contents = buf.toString();
  const ast = parse(contents, { parser: typescript }) as ASTNode;
  let found = 0;
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
        // This is not an object, so we cannot inspect its slug.
        return this.traverse(path);
      }

      // There may be multiple tasks in this file. Confirm this task's slug matches
      // the one we're updating.
      const slug = getStringValue(arg1, "slug");
      if (slug !== existingSlug) {
        return this.traverse(path);
      }
      found++;

      const cf = hasComputedFields(arg1);
      if (cf) {
        throw new Error("Tasks that use computed fields must be updated manually.");
      }

      const newNode = buildTaskConfig(def);
      node.arguments[0] = newNode;

      // Continue traversing.
      return this.traverse(path);
    },
  });

  if (found === 0) {
    throw new Error(`Could not find task with slug "${existingSlug}".`);
  } else if (found > 1) {
    throw new Error(`Found more than one task with slug "${existingSlug}".`);
  }

  // Return early without writing out any changes.
  if (opts.dryRun) {
    return;
  }

  let result = print(ast, {
    // When printing string literals, prefer the quote that will generate the shortest literal.
    quote: "auto",
  }).code;

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

const buildTaskConfig = (def: any): ExpressionKind => {
  // Rewrite the definition from JSON format into what is used by the JS SDK.
  //
  // The JS SDK does not have a "node" field since it's a JS task by definition.
  if (def.node) {
    // Environment variables are a top-level field, so bubble them up from within "node".
    if (def.node.envVars) {
      // Insert it via `entries` so that we can insert it where "node" was in the definition.
      const entries = Object.entries(def);
      const i = entries.findIndex((e) => e[0] === "node");
      entries.splice(i + 1, 0, ["envVars", def.node.envVars]);
      def = Object.fromEntries(entries);
    }

    delete def.node;
  }
  // Parameters are stored as a map of slug to parameter definition rather than a list of definitions.
  if (def.parameters) {
    const parameters = {};
    for (let p of def.parameters) {
      const { slug, ...rest } = p;
      if (Object.keys(rest).length === 1) {
        // Use the shorthand for parameters when only the type is set where the value
        // is the type instead of an object.
        parameters[slug] = rest.type;
      } else {
        // Convert default parameter values from JSON to JS values.
        if (rest.default != null) {
          rest.default = asRecastNode(buildParamValue(rest.default, rest.type));
        }
        // Convert option parameter values from JSON to JS values.
        if (rest.options) {
          rest.options = rest.options.map((option) => {
            const value = typeof option === "object" ? option.value : option;
            const rv = asRecastNode(buildParamValue(value, rest.type));
            return typeof option === "object" ? { ...option, value: rv } : rv;
          });
        }
        parameters[slug] = rest;
      }
    }
    def.parameters = parameters;
  }
  // Convert schedule parameter values from JSON to JS values.
  if (def.schedules) {
    for (const [sslug, schedule] of Object.entries<any>(def.schedules)) {
      if (schedule.paramValues) {
        for (const [pslug, value] of Object.entries<any>(schedule.paramValues)) {
          const param = def.parameters[pslug];
          const type = typeof param === "string" ? param : param.type;
          const newValue = buildParamValue(value, type);
          def.schedules[sslug].paramValues[pslug] = asRecastNode(newValue);
        }
      }
    }
  }

  return buildJSON(def);
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

type RecastNode = {
  __airplaneType: "recast_node";
  node: ExpressionKind;
};

const asRecastNode = (node: ExpressionKind): RecastNode => {
  return { __airplaneType: "recast_node", node };
};

const isRecastNode = (value: any): value is RecastNode => {
  return value && value.__airplaneType === "recast_node";
};

/**
 * Serialize a value as a JSON object.
 *
 * To override the serialization of a field value, wrap the value with a RecastNode using
 * `asRecastNode`. It's `.node` property will be used verbatim as the serialized value.
 */
const buildJSON = (value: any): ExpressionKind => {
  if (value == null) {
    return builders.nullLiteral();
  }
  if (isRecastNode(value)) {
    return value.node;
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
  if (namedTypes.StringLiteral.check(value)) {
    return value.value;
  }

  return "";
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
      continue;
    }

    if (keyName !== fieldName) {
      // This is not the property we want to update.
      continue;
    }

    return property.value;
  }

  return undefined;
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

// Returns `true` if the value `v` contains any non-literal values.
const hasComputedFields = (v: any): boolean => {
  if (namedTypes.ObjectExpression.check(v)) {
    for (const property of v.properties.values()) {
      if (namedTypes.ObjectProperty.check(property)) {
        // Continue...
      } else if (
        namedTypes.SpreadProperty.check(property) ||
        namedTypes.ObjectMethod.check(property) ||
        namedTypes.Property.check(property) ||
        namedTypes.SpreadElement.check(property)
      ) {
        return true;
      } else {
        return assertNever(property);
      }

      // e.g. `{ [name]: "value" }`
      if (property.computed) {
        return true;
      }

      if (
        namedTypes.StringLiteral.check(property.key) ||
        namedTypes.Identifier.check(property.key)
      ) {
        // Continue...
      } else {
        return true;
      }

      if (hasComputedFields(property.value)) {
        return true;
      }
    }

    return false;
  }

  if (namedTypes.ArrayExpression.check(v)) {
    for (const element of v.elements) {
      if (hasComputedFields(element)) {
        return true;
      }
    }

    return false;
  }

  if (
    namedTypes.NullLiteral.check(v) ||
    namedTypes.BooleanLiteral.check(v) ||
    namedTypes.NumericLiteral.check(v) ||
    namedTypes.StringLiteral.check(v)
  ) {
    return false;
  }

  if (namedTypes.TemplateLiteral.check(v)) {
    return v.expressions.length > 0;
  }

  if (namedTypes.TaggedTemplateExpression.check(v)) {
    return hasComputedFields(v.quasi);
  }

  if (namedTypes.Identifier.check(v) && v.name === "undefined") {
    return false;
  }

  return true;
};
