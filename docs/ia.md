# Information Architecture

## Main Frontend Routes

- `/[lang]`: DevBox list and home workspace view.
- `/[lang]/devbox/create`: create or edit a DevBox using form or YAML mode.
- `/[lang]/devbox/detail/[name]`: DevBox detail, lifecycle, network, release, logs, terminal, and monitoring surfaces.
- `/[lang]/template`: runtime/template repository management.
- `/api-docs`: API documentation surface.

## Important API Groups

- `/api/createDevbox`, `/api/updateDevbox`, `/api/startDevbox`, `/api/shutdownDevbox`, `/api/restartDevbox`: legacy internal frontend API facade.
- `/api/getDevboxList`, `/api/getDevboxByName`, `/api/getEnv`: internal data and environment routes.
- `/api/platform/*`: account, debt, domain, and resource price routes.
- `/api/templateRepository/*`: template repository and template management routes.
- `/api/v1/devbox/*`: v1-compatible public API routes.
- `/api/v2alpha/devbox/*`: v2 alpha public API routes.
- `/api/openapi` and `/api/v2alpha/openapi`: OpenAPI document routes.

## User Workflows

- Create DevBox: choose template, configure CPU/memory/storage/GPU, network, envs, ConfigMaps, and volumes, then submit.
- Edit DevBox: load existing CR state, adjust form/YAML, generate patch, optionally restart after ConfigMap changes.
- Connect IDE: choose enabled IDE option and launch local or web IDE connection flow.
- Manage lifecycle: start, stop, restart, shutdown, and delete DevBoxes.
- Release: create and deploy DevBox release images.
- Template management: list, create, edit, tag, and publish runtime templates.

## Sync Notes

During v1-to-v2 frontend sync work, compare behaviors at the workflow level rather than copying route or YAML shapes blindly. v2 pages and APIs may intentionally differ from v1 because v2 owns a different CRD and gateway contract.
