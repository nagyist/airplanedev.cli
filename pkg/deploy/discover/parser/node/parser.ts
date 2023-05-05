// After changing this file, run `yarn build` to build parser.js.

import path from "path";
import { JSDOM } from "jsdom";

type TaskParam = {
  slug: string;
  name: string;
  type: string;
  description?: string;
  default?: any;
  required?: boolean;
  options?: any[];
  regex?: string;
};

type NodeDef = {
  envVars?: Record<string, string | { config: string } | { value: string }>;
  entrypoint: string;
};

type TaskDef = {
  slug: string;
  node: NodeDef;
  name?: string;
  description?: string;
  parameters?: TaskParam[];
  requireRequests?: boolean;
  allowSelfApprovals?: boolean;
  restrictCallers?: string[];
  timeout?: number;
  constraints?: Record<string, string>;
  resources: Record<string, string> | string[];
  schedules: Record<string, any>;
  runtime?: "" | "workflow";
  defaultRunPermissions?: "task-viewers" | "task-participants";
  concurrencyKey?: string;
  concurrencyLimit?: number;
  sdkVersion?: string;
};

type TaskDefWithBuildArgs = TaskDef & {
  entrypointFunc: string;
};

type ViewDef = {
  slug: string;
  name: string;
  description?: string;
  entrypoint: string;
  envVars?: Record<string, string | { config: string } | { value: string }>;
};

type AirplaneConfigs = {
  taskConfigs: TaskDefWithBuildArgs[];
  viewConfigs: ViewDef[];
};

const extractTaskConfigs = (files: string[]): AirplaneConfigs => {
  let taskConfigs: TaskDefWithBuildArgs[] = [];
  let viewConfigs: ViewDef[] = [];
  for (const file of files) {
    const relPath = path.relative(__dirname, file);
    const exports = require(`./${relPath}`);

    for (const exportName in exports) {
      const item = exports[exportName];
      if (
        (typeof item === "object" || typeof item === "function") &&
        item !== null &&
        "__airplane" in item
      ) {
        const config = item.__airplane.config;
        if (item.__airplane.type === "view") {
          viewConfigs.push({
            slug: config.slug,
            // Default to slug if name is not provided.
            name: config.name || config.slug,
            description: config.description,
            entrypoint: file,
            envVars: config.envVars,
          });
        } else {
          const params: TaskParam[] = [];
          for (const uParamSlug in config.parameters) {
            const uParamConfig = config.parameters[uParamSlug];

            if (typeof uParamConfig === "string") {
              params.push({
                slug: uParamSlug,
                name: uParamSlug,
                type: uParamConfig,
              });
            } else {
              params.push({
                slug: uParamSlug,
                // Default to slug if name is not provided.
                name: uParamConfig["name"] || uParamSlug,
                type: uParamConfig["type"],
                description: uParamConfig["description"],
                default: uParamConfig["default"],
                required: uParamConfig["required"],
                options: uParamConfig["options"],
                regex: uParamConfig["regex"],
              });
            }
          }

          taskConfigs.push({
            slug: config.slug,
            name: config.name ?? config.slug,
            description: config.description,
            requireRequests: config.requireRequests,
            allowSelfApprovals: config.allowSelfApprovals,
            restrictCallers: config.restrictCallers,
            timeout: config.timeout,
            constraints: config.constraints,
            runtime: item.__airplane.type === "workflow" ? "workflow" : "",
            defaultRunPermissions: config.defaultRunPermissions,
            concurrencyKey: config.concurrencyKey,
            concurrencyLimit: config.concurrencyLimit,
            resources: config.resources,
            schedules: config.schedules,
            parameters: params,
            entrypointFunc: exportName,
            node: {
              envVars: config.envVars,
              entrypoint: file,
            },
            sdkVersion: config.sdkVersion,
          });
        }
      }
    }
  }
  return {
    taskConfigs,
    viewConfigs,
  };
};

const dom = new JSDOM(`<!DOCTYPE html><body></div></body>`);
// Add a document so that if the view contains frontend specific code that references the global document, the parser doesn't fail in a node environment.
(global as any).document = dom.window.document;
const files = process.argv.slice(2);
const taskConfigs = extractTaskConfigs(files);
console.log("EXTRACTED_ENTITY_CONFIGS:" + JSON.stringify(taskConfigs));
