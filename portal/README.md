# TogoAPI Portal

TogoAPI Portal is the public web entry for the Sub2API Core account pool.

## Ownership boundary

- Core owns authentication, users, API keys, balances, redemption, announcements, model routing, accounts, requests, usage, and billing facts.
- Portal provides the public web UI, a same-origin Core API adapter, username uniqueness, companies, company membership, scoped management, and finance aggregation.
- Portal never stores a second password system and never writes directly to the Core database.
- Portal owns the public six-digit password-reset code flow. It stores only short-lived HMAC digests and updates passwords through Core's scoped admin API.
- Core remains an independently upgradeable upstream Sub2API checkout.

## Local startup

Use the workspace-level launcher for normal development:

```powershell
cd ..
.\local.ps1 up
```

See [`../LOCAL_DEV.md`](../LOCAL_DEV.md) for environment setup, ports, health checks, and troubleshooting.

When debugging Portal by itself, first ensure Core is already running on `http://127.0.0.1:8080`, then run `docker compose --env-file .env up -d --build` in this directory. Do not override `CORE_BASE_URL` to a validation port.

The Portal API reaches Core through `host.docker.internal:8080`. Browser requests stay same-origin under `/api/*` and are proxied by the Portal web container.

## Production

Production uses `docker-compose.prod.yml` in addition to the base file. The override joins the Core network so Portal reaches Core by container DNS without exposing Core on a non-loopback host address.

```bash
cd /opt/portal
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env up -d --build
curl -fsS http://127.0.0.1:3000/health
```

Set `CORE_BASE_URL=http://sub2api:8080` in the production `.env`. The external network defaults to `sub2api_sub2api-network` and can be overridden with `CORE_DOCKER_NETWORK`. Keep `.env` mode `600` and never run `docker compose down -v`.
