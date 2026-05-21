# Devbox V2 Frontend References

## Internal References

- `README.md` - local frontend setup.
- `.env.template` - environment variable template.
- `deploy/manifests/deploy.yaml.tmpl` - deployment and migration container
  shape.
- `deploy/manifests/ingress.yaml.tmpl` - frontend and domain-challenge ingress
  shape.
- `services/backend/registry-retag.ts` - HTTPS-only registry retag helper.

## Related Notes

- v1 frontend has a fuller historical runbook in `../v1/frontend/docs/runbook.md`
  for cluster 70 style operations.
- Registry retag configuration is frontend-owned for template repository
  create/update flows. Controller registry transport is a separate component
  boundary.
