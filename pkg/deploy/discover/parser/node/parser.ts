const path = require("path");

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
  nodeVersion: "12" | "14" | "15" | "16" | "18";
  envVars?: Record<string, string | { config: string } | { value: string }>;
}

type TaskDef = {
  slug: string;
  node: NodeDef;
  name?: string;
  description?: string;
  parameters?: TaskParam[];
  requireRequests?: boolean;
  allowSelfApprovals?: boolean;
  timeout?: number;
  constraints?: Record<string, string>;
  resources: Record<string, string> | string[];
  schedules: Record<string, any>;
  runtime?: "" | "workflow";
};

type TaskDefWithBuildArgs = TaskDef & {
  entrypointFunc: string;
};

type ViewDef = {
  slug: string;
  name: string;
  description?: string;
}

type AirplaneConfigs = {
  taskConfigs: TaskDefWithBuildArgs[];
  viewConfigs: ViewDef[];
}

const extractTaskConfigs = (files: string[]): AirplaneConfigs => {
  let taskConfigs: TaskDefWithBuildArgs[] = [];
  let viewConfigs: ViewDef[] = [];
  for (const file of files) {
    const relPath = path.relative(__dirname, file);
    const exports = require(`./${relPath}`);

    for (const exportName in exports) {
      const item = exports[exportName];

      if ("__airplane" in item) {
        const config = item.__airplane.config;
        if (item.__airplane.type === "view") {
          viewConfigs.push({
            slug: config.slug,
            name: config.name,
            description: config.description,
          })
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
                name: uParamConfig["name"] ? uParamConfig["name"] : uParamSlug,
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
            timeout: config.timeout,
            constraints: config.constraints,
            runtime: config.runtime === "workflow" ? "workflow" : "",
            resources: config.resources,
            schedules: config.schedules,
            parameters: params,
            entrypointFunc: exportName,
            node: {
              nodeVersion: config.nodeVersion ?? "18",
              envVars: config.envVars,
            }
          });
        }
      }
    }
  }
  return {
    taskConfigs,
    viewConfigs,
  }
};

const files = process.argv.slice(2);
const taskConfigs = extractTaskConfigs(files);
console.log(JSON.stringify(taskConfigs));
