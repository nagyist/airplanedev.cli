# Contributing to the Airplane lib

## Tagging

To tag a new version of the Airplane lib, run the following command with the new tag you want to create:

```sh
export AIRPLANE_LIB_TAG=v0.0.1 && \
  git tag ${AIRPLANE_LIB_TAG} && \
  git push origin ${AIRPLANE_LIB_TAG}
```

## Base images

If you make changes to the base images -- [`pkg/build/versions.json`](/pkg/build/versions.json) -- then you'll need to push these into the public cache by running `scripts/cache/main.airplane.ts` in both `airplane-stage` and `airplane-prod`.

You will need to run this locally so it can access your GCP credentials:

```sh
# You may need to temporarily remove a few fixtures for this to work.
# rm -rf pkg && git checkout $(git rev-parse --abbrev-ref HEAD) pkg/build/versions.json
# To revert:
# git checkout main pkg
airplane dev
```
