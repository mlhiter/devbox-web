# Devbox V2 Frontend Information Architecture

## Primary Pages

- `/[lang]` - DevBox list/home page.
- `/[lang]/devbox/create` - create form.
- `/[lang]/devbox/detail/[name]` - DevBox detail page.
- `/[lang]/template` - template repository page.
- `/api-docs` - API docs surface.
- `/api/v2alpha/docs` - v2alpha API docs surface.

## Main User Flows

- List DevBoxes -> create DevBox -> open IDE.
- Detail page -> manage network and access configuration.
- Detail page -> release -> deploy app.
- Detail page or template page -> create/update template repository from a
  DevBox release.
- Import from Git or local archive when import features are enabled.

## Major API Groups

- DevBox CRUD and lifecycle: `getDevboxList`, `getDevboxByName`,
  `createDevbox`, `updateDevbox`, `startDevbox`, `shutdownDevbox`,
  `restartDevbox`, `delDevbox`.
- Network and access: `getDevboxPorts`, `updateDevboxWebIDEPort`,
  `platform/authCname`, `platform/authDomainChallenge`, `checkReady`.
- Release and deployment: `releaseDevbox`, `releaseAndDeployDevbox`,
  `deployDevbox`.
- Templates: `templateRepository/*`.
- Platform support: guide routes, monitor routes, OpenAPI routes, platform
  pricing and debt routes.

## Navigation Notes

Template repository create/update actions depend on server-side registry retag
support. They require `REGISTRY_ADDR`, `REGISTRY_USER`, and `REGISTRY_PASSWORD`
to be configured for an HTTPS-capable registry.
