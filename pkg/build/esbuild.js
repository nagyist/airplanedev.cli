const esbuild = require("esbuild");
const fs = require("fs");

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
    external: [...external],
    outdir,
    outbase,
  })
  .catch((e) => {
    process.exit(1);
  });
