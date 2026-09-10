# TogoAPI

Server access, deployed infrastructure, Cloudflare and DNS configuration, HTTPS, firewall, incident status, and recovery checklist: [SERVER.md](./SERVER.md)

Canonical local startup and troubleshooting: [LOCAL_DEV.md](./LOCAL_DEV.md)

This workspace separates the public Portal from the Sub2API Core account pool.

```text
Browser -> Portal Web :3000 -> Portal API -> Core :8080
                                  |             |
                                  v             +-> Core PostgreSQL / Redis
                           Portal PostgreSQL
```

- `portal/`: TogoAPI user entry, API adapter, company and finance extensions.
- `core/`: local checkout of upstream Sub2API, responsible for the account pool and all execution facts. It keeps its own git history and is not stored in this repository.

Core can be upgraded independently. Portal must use Core's official APIs and must not read or write the Core database directly.

For normal local development, run the following from the workspace root. Do not start separate clean or validation stacks alongside it.

```powershell
.\local.ps1 up
```

Open `http://127.0.0.1:3000` for Portal and `http://127.0.0.1:8080` for Core.

Public topology:

- `https://api.togoapi.com`: deployed Sub2API Core and `/v1` API.
- `https://togoapi.com`: deployed Portal, including the Core API adapter and Imgtool SSO entry.
- `https://img.togoapi.com`: deployed Imgtool frontend and API; direct visits require entry through Portal SSO.
- `https://togoapi.com/gift/`: deployed lucky-code campaign.
- `https://togoapi.com/lottery/`: deployed lottery campaign.

Operational note (2026-09-10): production traffic uses the Tokyo origin `43.165.190.2` through three proxied Cloudflare A records (`@`, `api`, and `img`). Cloudflare real-IP handling, request-header hardening, and observation-only ingress limits are active on Nginx. Core, Portal, Imgtool, Gift, and Lottery are deployed and healthy on Tokyo. AIPPT, the QQ status bot, and kiro-rs were stopped on 2026-09-10 and must not be restarted. Maintenance SSH uses key-only port `22` through the `togoapi-tokyo` alias. The former Singapore application services are stopped; its PostgreSQL containers are retained temporarily for rollback. See `SERVER.md` before changing Nginx, DNS, Cloudflare, or firewall configuration.

Registration email uses Alibaba Cloud DirectMail over TLS SMTP. The sending domain and local delivery test are complete; production SMTP setup remains on the verification checklist in `SERVER.md`.

Payment pricing rule: TogoAPI intentionally grants `1 USD` balance for each `1 CNY` paid. Keep the balance recharge multiplier at `1`; see `SERVER.md` before changing payment conversion settings.
