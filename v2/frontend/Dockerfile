# Install dependencies only when needed
ARG NODE_IMAGE=node:20.4.0-alpine
ARG PNPM_VERSION=10.32.1

FROM ${NODE_IMAGE} AS deps
ARG PNPM_VERSION
# Check https://github.com/nodejs/docker-node/tree/b4117f9333da4138b03a546ec926ef50a31506c3#nodealpine to understand why libc6-compat might be needed.
RUN apk add --no-cache libc6-compat openssl && npm install -g pnpm@${PNPM_VERSION}
WORKDIR /app

# Install dependencies based on the preferred package manager
COPY package.json pnpm-lock.yaml* ./
COPY prisma ./prisma
RUN if [ -f pnpm-lock.yaml ]; then \
    pnpm install; \
  else \
    echo "Lockfile not found."; \
    exit 1; \
  fi

# Rebuild the source code only when needed
FROM ${NODE_IMAGE} AS builder
ARG PNPM_VERSION
WORKDIR /app
RUN apk add --no-cache openssl
COPY --from=deps /app/node_modules ./node_modules
COPY . .
COPY --from=deps /app/prisma/generated ./prisma/generated

# Next.js collects completely anonymous telemetry data about general usage.
# Learn more here: https://nextjs.org/telemetry
# Uncomment the following line in case you want to disable telemetry during the build.
ENV NEXT_TELEMETRY_DISABLED 1
ENV NEXT_PUBLIC_MOCK_USER ''

RUN npm install -g pnpm@${PNPM_VERSION} && pnpm run build

# Production image, copy all the files and run next
FROM ${NODE_IMAGE} AS runner
WORKDIR /app

ENV NODE_ENV production
# Uncomment the following line in case you want to disable telemetry during runtime.
ENV NEXT_TELEMETRY_DISABLED 1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

RUN sed -i 's/https/http/' /etc/apk/repositories
RUN apk add --no-cache curl ca-certificates openssl \
  && (apk add --no-cache postgresql18-client \
    || apk add --no-cache postgresql17-client \
    || apk add --no-cache postgresql16-client \
    || apk add --no-cache postgresql15-client \
    || apk add --no-cache postgresql-client) \
  && update-ca-certificates

# You only need to copy next.config.js if you are NOT using the default configuration
# COPY --from=builder /app/next.config.js ./
COPY --from=builder /app/package.json ./package.json
# COPY --from=builder /app/.env* .

# Automatically leverage output traces to reduce image size
# https://nextjs.org/docs/advanced-features/output-file-tracing
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone/app ./
COPY --from=builder --chown=nextjs:nodejs /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000

CMD ["node", "server.js"]
