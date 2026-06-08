# Devbox V2 Frontend References

## Internal References

- `README.md` - local frontend setup.
- `.env.template` - environment variable template.
- `deploy/Kubefile` - Sealos package entrypoint for the frontend chart.
- `deploy/install.sh` - cluster install script that reads Sealos global and app
  values before running Helm.
- `deploy/charts/devbox-v2-frontend` - Helm chart for the frontend Deployment,
  runtime Secret, Service, Ingresses, and App CR.
- `services/backend/registry-retag.ts` - HTTPS-only registry retag helper.

## Related Notes

- v1 frontend has a fuller historical runbook in `../v1/frontend/docs/runbook.md`
  for cluster 70 style operations.
- Registry retag configuration is frontend-owned for template repository
  create/update flows. Controller registry transport is a separate component
  boundary.
