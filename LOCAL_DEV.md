# Local Development

This is the canonical local environment for TogoAPI. Normal development uses exactly one Core stack and one Portal stack.

```text
Browser -> Portal Web :3000 -> Portal API -> Core :8080
                                  |             |
                                  v             +-> Core PostgreSQL / Redis
                           Portal PostgreSQL
```

## Ports

| Service | URL | Notes |
| --- | --- | --- |
| Portal | `http://127.0.0.1:3000` | The normal browser entry point |
| Core | `http://127.0.0.1:8080` | Core UI and API |
| Portal API | Docker internal only | Reached through Portal `/api/*` |
| PostgreSQL / Redis | Docker internal only | Do not publish during normal development |

Ports `18080`, `18081`, `18180`, and `18181` are reserved for temporary tests. Validation stacks using these ports must be stopped after the test and must not be used by the normal Portal.

## First-time setup

Create the two ignored environment files if they do not exist:

```powershell
Copy-Item core\deploy\.env.example core\deploy\.env
Copy-Item portal\.env.example portal\.env
```

Set strong local secrets. Portal must contain this exact Core address:

```dotenv
CORE_BASE_URL=http://host.docker.internal:8080
```

The Portal `CORE_ADMIN_API_KEY` must be an active Admin API Key created in the same Core instance running on port `8080`.

If Docker Hub is unavailable, uncomment the `PORTAL_*_IMAGE` mirror overrides documented in `portal/.env.example`. Keep mirror selection in the ignored local `.env`; do not hard-code a regional mirror in the Compose file.

## Commands

Run all commands from the workspace root:

```powershell
# Build and start Core first, then Portal, and wait for both health checks.
.\local.ps1 up

# Show only the canonical Core and Portal stacks.
.\local.ps1 status

# Rebuild Portal and restart the canonical environment.
.\local.ps1 restart

# Show recent Portal and Core logs.
.\local.ps1 logs

# Stop the canonical environment without deleting data volumes.
.\local.ps1 down
```

If Docker Hub or the configured registry is temporarily unavailable, use the last successfully built Portal API image plus a fresh host-side frontend build:

```powershell
.\local.ps1 up -UseBuiltArtifacts
```

This fallback still uses the same Portal Compose project, database, ports, and Core target. It does not create a second environment.

Open `http://127.0.0.1:3000`. Core accounts are used directly by Portal; Portal does not maintain a second password database. The initial administrator normally signs in with the email configured as `ADMIN_EMAIL`, not the word `admin`.

## Rules

- Use `local.ps1` for normal local work. Do not start ad-hoc containers for Portal or Core.
- Do not point Portal at `18080` or another validation Core.
- Do not keep validation, rollback, or smoke-test Compose projects running after a test.
- Do not run `docker compose down -v`; it deletes local databases.
- Do not expose PostgreSQL or Redis ports unless a focused debugging task requires it, and remove the exposure afterward.
- Do not commit `.env` files or copy secrets into documentation.

## Troubleshooting

Confirm the effective Portal target:

```powershell
docker compose --env-file portal\.env -f portal\docker-compose.yml config |
  Select-String 'CORE_BASE_URL'
```

Confirm the end-to-end public settings route:

```powershell
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:3000/api/v1/settings/public
```

If login returns HTTP `429`, stop retrying and wait one minute. Core limits login attempts to 20 per minute per client key.
