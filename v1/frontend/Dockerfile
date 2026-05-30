# Install dependencies only when needed
FROM node:20.4.0-alpine AS deps
# Check https://github.com/nodejs/docker-node/tree/b4117f9333da4138b03a546ec926ef50a31506c3#nodealpine to understand why libc6-compat might be needed.
RUN apk add --no-cache libc6-compat && npm install -g pnpm@10.32.1
WORKDIR /workspace/frontend

# Install dependencies based on the preferred package manager
COPY package.json pnpm-lock.yaml* ./
COPY prisma ./prisma
RUN \
  [ -f pnpm-lock.yaml ] && pnpm install || \
  (echo "Lockfile not found." && exit 1)

# Rebuild the source code only when needed
FROM node:20.4.0-alpine AS builder
WORKDIR /workspace/frontend
COPY --from=deps /workspace/frontend/node_modules ./node_modules
COPY . .
COPY --from=deps /workspace/frontend/prisma/generated ./prisma/generated

# Next.js collects completely anonymous telemetry data about general usage.
# Learn more here: https://nextjs.org/telemetry
# Uncomment the following line in case you want to disable telemetry during the build.
ENV NEXT_TELEMETRY_DISABLED 1
ENV NEXT_PUBLIC_MOCK_USER ''
ARG DEVBOX_COMMIT_HASH=source-unavailable
ENV NEXT_PUBLIC_DEVBOX_COMMIT_HASH=${DEVBOX_COMMIT_HASH}
ENV NODE_OPTIONS=--max-old-space-size=4096

RUN npm install -g pnpm@10.32.1 && pnpm run build

# Production image, copy all the files and run next
FROM node:20.4.0-alpine AS runner
WORKDIR /app

ENV NODE_ENV production
# Uncomment the following line in case you want to disable telemetry during runtime.
ENV NEXT_TELEMETRY_DISABLED 1
ENV PATH="/app/node_modules/.bin:${PATH}"

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

RUN sed -i 's/https/http/' /etc/apk/repositories
RUN apk add --no-cache curl ca-certificates \
  && (apk add --no-cache postgresql18-client \
    || apk add --no-cache postgresql17-client \
    || apk add --no-cache postgresql16-client \
    || apk add --no-cache postgresql15-client \
    || apk add --no-cache postgresql-client) \
  && update-ca-certificates

# You only need to copy next.config.js if you are NOT using the default configuration
# COPY --from=builder /app/next.config.js ./
COPY --from=builder /workspace/frontend/public ./public
COPY --from=builder /workspace/frontend/package.json ./package.json
# COPY --from=builder /app/.env* .

# Automatically leverage output traces to reduce image size.
# outputFileTracingRoot is the workspace parent, so standalone preserves this
# project as "frontend"; copy its contents to /app for a simple runtime entry.
COPY --from=builder --chown=nextjs:nodejs /workspace/frontend/.next/standalone/frontend ./
COPY --from=builder --chown=nextjs:nodejs /workspace/frontend/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /workspace/frontend/node_modules ./node_modules
# Next standalone includes prisma schemas/clients but not migrations.
# Keep both providers' migrations for initContainer migrate deploy.
COPY --from=builder --chown=nextjs:nodejs /workspace/frontend/prisma/cockroach/schema.prisma ./prisma/cockroach/schema.prisma
COPY --from=builder --chown=nextjs:nodejs /workspace/frontend/prisma/cockroach/migrations ./prisma/cockroach/migrations
COPY --from=builder --chown=nextjs:nodejs /workspace/frontend/prisma/postgresql/schema.prisma ./prisma/postgresql/schema.prisma
COPY --from=builder --chown=nextjs:nodejs /workspace/frontend/prisma/postgresql/migrations ./prisma/postgresql/migrations

RUN test -x /app/node_modules/.bin/prisma && prisma --version

USER nextjs

EXPOSE 3000

CMD ["node", "server.js"]
