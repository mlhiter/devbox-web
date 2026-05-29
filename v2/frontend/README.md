## Package source

- Frontend dependencies are installed from npm registry.
- Use published `@labring/*` packages directly (for example `@labring/sealos-ui`, `@labring/sealos-driver-sdk`, `@labring/sealos-desktop-sdk`).
- Do not use `yalc link` / `yalc remove` in this repo.
- Frontend-side template image retagging requires HTTPS. Configure
  `REGISTRY_ADDR`, `REGISTRY_USER`, and `REGISTRY_PASSWORD`; do not use
  `REGISTRY_INSECURE` or HTTP registry endpoints.

## How to dev

1. Install dependencies and start from this directory:

   ```bash
   cd /Users/mlhiter/labring/devbox/v2/frontend
   pnpm install
   ```

2. Then you should config your env.

   1. Create a new file `.env.local` in `v2/frontend`.

      > `SEALOS_DOMAIN` is anyone website you use in sealos.

   2. ```
      NEXT_PUBLIC_MOCK_USER=""
      SEALOS_DOMAIN="bja.sealos.run"
      NODE_ENV="development"
      ```

   3. Then we should get our `NEXT_PUBLIC_MOCK_USER`

   4. go to `bja.sealos.run` and login (firstly you goto this website,sign up,so go on.)

   5. Refer this picture,you should open `Console-application`，and get your own `session.state.session.kubeconfig`,copy as JSON string.

      ![image-20240423105724369](https://raw.githubusercontent.com/mlhiter/typora-images/master/202404231101028.png)

3. Start the local dev server.

   ```bash
   pnpm dev
   ```

   This listens on `localhost:3108` by default to avoid local port 3000
   collisions with other development tools. If you need the Sealos Desktop
   wrapper that expects `localhost:3000`, use:

   ```bash
   pnpm dev:desktop
   ```

4. After that,have your own `test 3000 `page

   > Why you should have that?
   >
   > If you open your own dev in `localhost:3000` directly,you cannot have sealos desktop border,which maybe influence your style.

   1. This url：[website](https://cloud.sealos.run/?openapp=system-template%3FtemplateName%3Done-step-shortcuts)

   2. ![image-20240423111024336](https://raw.githubusercontent.com/mlhiter/typora-images/master/202404231110609.png)

   3. Refresh website.

   4. Then you can get your own dev in this.

      ![image-20240423111123308](https://raw.githubusercontent.com/mlhiter/typora-images/master/202404231111720.png)
