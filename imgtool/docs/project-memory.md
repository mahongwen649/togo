# Project Memory

## Goal

Build a web version of the existing image generation desktop tool.

## Chosen Direction

Use "方案 B+":

- Private web app with simple account isolation.
- One fixed `admin` account.
- `admin` creates and manages normal accounts.
- No public registration.
- Each account owns its own channels, API keys, generation history, and images.
- Minimal authentication is required and should be enforced on pages and APIs.

## Authentication Requirements

- Username + password login.
- HttpOnly cookie session.
- Server-side session validation for all protected pages and API routes.
- Password hash storage.
- No OAuth, email verification, teams, billing, or complex RBAC.
- `admin` is the only admin role.

## Channel/API Key Requirements

- Channels and API keys are per user.
- API keys move from browser localStorage to server-side SQLite.
- API keys should be encrypted at rest with `IMGTOOL_SECRET`.
- Frontend generation requests should not send raw API keys.
- Frontend should submit channel/model/capability selection.
- Backend should resolve the selected channel for the current logged-in user.

## Image Storage Requirements

- Use Aliyun OSS, not local disk, for generated image files.
- Bucket: `whalesing-web`
- Prefix: `aiImg`
- Endpoint: `https://oss-cn-guangzhou.aliyuncs.com`
- Target root: `oss://whalesing-web/aiImg/`
- Bucket should remain private.
- Backend should create short-lived signed URLs after verifying ownership.
- Suggested object key pattern:
  `aiImg/users/{userId}/{channelSlug}/{modelSlug}/{YYYY-MM-DD}/{HHmmss}-{taskId}-{index}.{ext}`

## Local Sensitive Files

- `oss.txt` exists locally and contains OSS credentials.
- Do not commit `oss.txt`.
- Do not repeat credentials in docs or final responses.
- Use `.env` during implementation.

## Existing App Notes

Existing desktop app path:

```text
D:\tool
```

Useful reusable parts:

- `app/page.tsx`
- `app/settings/page.tsx`
- `app/history/page.tsx`
- `app/api/*`
- `lib/server/adapters.ts`
- `lib/server/history-store.ts`
- `lib/server/validation.ts`
- `lib/server/redaction.ts`
- `lib/server/format-upstream-error.ts`
- tests around adapters, file store, redaction, validation, and model helpers

Parts to remove or replace:

- Electron shell and desktop packaging.
- localStorage channel storage.
- local filesystem output storage.
- API shape that passes raw `apiKey` from frontend to backend.

## Important Caveat

`D:\tool` has uncommitted changes. Treat it as a read-only source until the user confirms how to handle those changes.
