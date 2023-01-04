const esbuild = require("esbuild");
const fs = require("fs");

const jsdomPatch = {
  name: "jsdom-patch",
  setup(build) {
    build.onLoad({ filter: /XMLHttpRequest-impl\.js$/ }, async (args) => {
      let contents = await fs.promises.readFile(args.path, "utf8");
      contents = contents.replace(
        'const syncWorkerFile = require.resolve ? require.resolve("./xhr-sync-worker.js") : null;',
        `const syncWorkerFile = "${require.resolve(
          "jsdom/lib/jsdom/living/xhr/xhr-sync-worker.js"
        )}";`
      );
      return { contents, loader: "js" };
    });
  },
};

const entryPoints = JSON.parse(process.argv[2]);
const target = process.argv[3];
const external = JSON.parse(process.argv[4]);
const outfile = process.argv[5] || undefined;
const outdir = process.argv[6] || undefined;
const outbase = process.argv[7] || undefined;

esbuild
  .build({
    entryPoints,
    outfile,
    bundle: true,
    target,
    platform: "node",
    external: [...external, "canvas"],
    outdir,
    outbase,
    plugins: [jsdomPatch],
  })
  .catch((e) => {
    process.exit(1);
  });
