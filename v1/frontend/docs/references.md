# Devbox References

## Internal References

- `README.md` - local frontend setup.
- `prisma/README.md` - Prisma schema layout.
- `.env.template` - environment variable template.
- `deploy/manifests/deploy.yaml.tmpl` - deployment and migration container
  shape.
- `deploy/manifests/ingress.yaml.tmpl` - frontend and
  domain-challenge ingress shape.

## Issue References

- `labring-sigs/sealos-issues#171` - custom domain drawer appeared to do nothing
  when DNS validation failed. The fix maps dynamic DNS errors to stable UI copy
  and avoids sending raw DNS strings through `next-intl`.
