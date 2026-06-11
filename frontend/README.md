# ZBT Frontend

React + TypeScript + Vite + Ant Design frontend for 智标通.

## Local Commands

```bash
pnpm install
pnpm dev --host 0.0.0.0
pnpm build
```

The Docker entrypoint serves the production build through Nginx at `http://localhost:5173` and proxies `/api/v1` to the Go backend.

## Runtime Contract

- API base defaults to `/api/v1`.
- Session state is stored in `src/app/store/session.ts`.
- Navigation is declared in `src/routes/routeManifest.tsx`.
- Menus and route guards both use module permissions from the current login session.
- Direct URL access without permission renders the 403 page.

Use the root [README](../README.md) for full-stack startup and validation commands.
