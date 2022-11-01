import execa from 'execa'
import * as fs from 'fs'
import airplane from 'airplane'

export default airplane.task({
  slug: "update_registry_cache",
  name: 'Update Registry Cache',
  description: 'This ensures the Airplane Registry hosts all of the latest base images in its public cache.',
  parameters: {
    gcp_project: {
      name: "GCP project",
      description: "The ID (e.g. `airplane-prod`) to publish to.",
      type: "shorttext",
      options: ["airplane-prod", "airplane-stage"],
      default: "airplane-stage",
    }
  },
}, async (params) => {
  if (!params.gcp_project) {
    throw new Error("expected a project")
  }

  const versionsFile = JSON.parse(fs.readFileSync('./pkg/build/versions.json') as any) as Record<string, Record<string, {
    image: string
    tag: string
    digest: string
  }>>
  console.log(versionsFile)

  for (let kind of Object.keys(versionsFile)) {
    for (let version of Object.values(versionsFile[kind])) {
      airplane.appendOutput(`Pushing ${kind}:${version.tag}@${version.digest} to ${params.gcp_project}...`, "logs")
      // Digests should always match the tag, to make it easier to read `versions.json`.
      // The digest should be for linux/amd64.
      await execa("docker", ["pull", `${version.image}@${version.digest}`])
      await execa("docker", ["tag", `${version.image}@${version.digest}`, `us-central1-docker.pkg.dev/${params.gcp_project}/public-cache/${kind}:${version.tag}`])
      await execa("docker", ["push", `us-central1-docker.pkg.dev/${params.gcp_project}/public-cache/${kind}:${version.tag}`])
      airplane.appendOutput(`Pushed ${kind}:${version.tag}@${version.digest} to ${params.gcp_project}`, "logs")
    }
  }
})
