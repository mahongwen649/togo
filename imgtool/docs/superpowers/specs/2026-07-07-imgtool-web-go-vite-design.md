# Imgtool Web Go + Vite Design

## Summary

Build the first web MVP as a private multi-account application using a Vite React frontend and a Go backend. The old desktop app at `D:\tool` remains a read-only reference. The new project at `D:\code\aicode\ai\imgtool` will reuse the existing React UI ideas, CSS, model helper behavior, validation rules, and provider adapter behavior where practical, but the runtime will no longer be Next.js or Electron.

The MVP must provide a complete vertical workflow:

- Username/password login with HttpOnly cookie sessions.
- A fixed admin account that can create users, disable users, and reset passwords.
- Per-user channel and API key configuration.
- Image generation and image-to-text generation through the user's own configured channels.
- SQLite-backed tasks, history, channel configuration, sessions, and file metadata.
- Aliyun OSS storage for generated image files.
- Strict user isolation for channels, tasks, history records, and image access.

## Goals

- Replace the desktop app shell with a real browser-based app.
- Replace the Next.js full-stack runtime with a Vite React SPA and Go API server.
- Keep the first release private and account-isolated, not SaaS.
- Avoid public registration, OAuth, email flows, billing, teams, or complex RBAC.
- Keep frontend code from ever seeing raw configured API keys after channel creation/update.
- Keep OSS credentials on the server only.
- Keep generated image binaries out of SQLite and local app storage.

## Non-Goals

- No self-registration.
- No OAuth or external identity provider.
- No email verification or password recovery email.
- No team/organization permission model.
- No billing, quota sales, or subscription logic.
- No multi-node distributed task queue in the first implementation.
- No migration of existing desktop `data/` or `outputs/` content unless requested later.
- No modifications to `D:\tool` during this phase.

## Technology Choices

### Frontend

- Vite
- React
- TypeScript
- React Router for client-side routing
- Existing CSS and layout ideas from the desktop React pages
- `lucide-react` for icons

The frontend is a logged-in workspace app, not an SEO or SSR site. A SPA is simpler than keeping Next.js only for pages and API routes that are being replaced by Go.

### Backend

- Go HTTP server
- SQLite database
- Standard library `net/http` routing
- HttpOnly cookie sessions
- Argon2id password hashing via `golang.org/x/crypto/argon2`
- API key encryption using `IMGTOOL_SECRET`
- Pure Go SQLite driver, preferably `modernc.org/sqlite`, to keep Windows builds simple
- Aliyun OSS SDK or a small OSS client wrapper
- Embedded static frontend assets for production

The Go server owns authentication, authorization, channels, task persistence, provider calls, OSS upload, and signed URL generation. In production it serves the built Vite frontend from embedded `web/dist` assets.

## Directory Layout

Use a two-part application layout:

```text
imgtool/
  docs/
  web/
    src/
    public/
    index.html
    package.json
    vite.config.ts
    tsconfig.json
  server/
    cmd/imgtool/
      main.go
    internal/
      auth/
      channels/
      config/
      crypto/
      db/
      generation/
      httpapi/
      history/
      oss/
      users/
    go.mod
    go.sum
  .env.example
  .gitignore
```

The exact package names can evolve during implementation, but the design intent is stable:

- `web/` contains only browser code.
- `server/` contains Go code and all privileged server behavior.
- `docs/` contains project memory, configuration, specs, and plans.

## Configuration

Use environment variables for secrets and deployment-specific settings:

```env
IMGTOOL_ENV=development
IMGTOOL_HTTP_ADDR=127.0.0.1:8080
IMGTOOL_DATABASE_PATH=./data/imgtool.sqlite
IMGTOOL_SECRET=change-me
IMGTOOL_ADMIN_USERNAME=admin
IMGTOOL_ADMIN_PASSWORD=change-me-on-first-run

IMGTOOL_STORAGE_PROVIDER=aliyun-oss
ALIYUN_OSS_BUCKET=whalesing-web
ALIYUN_OSS_ENDPOINT=https://oss-cn-guangzhou.aliyuncs.com
ALIYUN_OSS_PREFIX=aiImg
ALIYUN_OSS_SIGNED_URL_EXPIRES=600
ALIYUN_OSS_ACCESS_KEY_ID=
ALIYUN_OSS_ACCESS_KEY_SECRET=
```

`oss.txt` may be used locally as the source for OSS credentials, but its contents must not be copied into docs, logs, test fixtures, or committed files. `.env`, `.env.*`, and `oss.txt` stay ignored.

## Authentication Design

Users sign in with username and password. The server verifies the password hash and creates a session row. The session id is sent in an HttpOnly cookie.

Cookie policy:

- `HttpOnly`
- `SameSite=Lax`
- `Path=/`
- `Secure` when `IMGTOOL_ENV=production`

Protected API routes load the current session and user before handling the request. Disabled users cannot use existing sessions. Logout deletes the session row and expires the cookie.

Admin bootstrapping:

- On startup or migration, if no admin user exists, create one using `IMGTOOL_ADMIN_USERNAME` and `IMGTOOL_ADMIN_PASSWORD`.
- The password is hashed before storage.
- The server must not expose the admin password to the frontend.

Roles:

- `admin`
- `user`

Only `admin` can call user-management APIs. There is no broader RBAC model in the MVP.

## Database Design

SQLite stores all metadata. Tables use millisecond Unix timestamps or RFC3339 timestamps consistently; implementation can choose one convention and apply it everywhere.

### `users`

- `id` primary key
- `username` unique
- `password_hash`
- `role` check `admin` or `user`
- `disabled_at` nullable
- `created_at`
- `updated_at`

### `sessions`

- `id` primary key
- `user_id`
- `expires_at`
- `created_at`
- `last_seen_at`

### `channels`

- `id` primary key
- `user_id`
- `name`
- `base_url`
- `encrypted_api_key`
- `created_at`
- `updated_at`
- unique `(user_id, name)`

### `channel_models`

- `id` primary key
- `channel_id`
- `model_id`
- `capabilities_json`
- `created_at`
- `updated_at`
- unique `(channel_id, model_id)`

### `tasks`

- `id` primary key
- `user_id`
- `kind` check `image`, `video`, or `text`
- `status` check `running`, `completed`, `failed`, or `timeout`
- `channel_id`
- `channel_name`
- `base_url`
- `model_id`
- `capability`
- `prompt`
- `parameters_json`
- `upstream_task_id` nullable
- `started_at`
- `ended_at` nullable
- `duration_ms` nullable
- `error_source` nullable
- `failure_reason` nullable
- `created_at`
- `updated_at`

Channel snapshots are copied into task rows so history remains readable if a channel is renamed or deleted.

### `history_records`

- `id` primary key
- `user_id`
- `task_id` unique
- `kind`
- `status` check `完成`, `失败`, or `超时`
- `channel_id`
- `channel_name`
- `base_url`
- `model_id`
- `capability`
- `prompt`
- `parameters_json`
- `started_at`
- `ended_at`
- `duration_ms`
- `error_source` nullable
- `failure_reason` nullable
- `result_text` nullable
- `created_at`
- `deleted_at` nullable

### `result_files`

- `id` primary key
- `user_id`
- `history_id`
- `storage_provider`
- `bucket`
- `object_key`
- `media_type` check `image`
- `mime_type`
- `size_bytes`
- `created_at`

Every file lookup must verify `result_files.user_id` against the current authenticated user.

## API Design

All API responses use a stable envelope:

```json
{ "ok": true, "data": {} }
```

or:

```json
{
  "ok": false,
  "error": {
    "source": "proxy",
    "code": "invalid_parameters",
    "message": "..."
  }
}
```

### Auth

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/me`

### Admin Users

- `GET /api/admin/users`
- `POST /api/admin/users`
- `PATCH /api/admin/users/{id}`
- `POST /api/admin/users/{id}/reset-password`

Admin can create users, disable/enable users, and reset passwords. Admin cannot delete users in the MVP.

### Channels

- `GET /api/channels`
- `POST /api/channels`
- `PATCH /api/channels/{id}`
- `DELETE /api/channels/{id}`

The frontend can submit an API key when creating or replacing a channel key. The server never returns the raw API key. Channel responses include only metadata such as `hasApiKey`.

### Generation

- `POST /api/generate/image`
- `POST /api/generate/text`

Generation requests are `multipart/form-data`.

Image generation fields:

- `channelId`
- `modelId`
- `capability`
- `prompt`
- `parameters`
- optional `inputImage`

Image-to-text fields:

- `channelId`
- `modelId`
- `capability=image-to-text`
- `prompt`
- required `inputImage`

The frontend does not send `baseUrl`, channel name, or API key. The server resolves the channel by `channelId` and current `user_id`, validates that `modelId` belongs to the channel, decrypts the API key, and calls the provider adapter.

### Tasks And History

- `GET /api/tasks`
- `GET /api/tasks/{id}`
- `POST /api/tasks/{id}/cancel`
- `GET /api/history`
- `DELETE /api/history/{id}`

Every query filters by current `user_id`. Cancelling a task marks local state as cancelled or failed if the upstream provider cannot be cancelled.

### Files

- `GET /api/files/{id}/url`

The server verifies the file belongs to the current user, then returns a short-lived signed OSS URL.

```json
{
  "ok": true,
  "data": {
    "url": "https://...",
    "expiresAt": "..."
  }
}
```

The default expiry is `ALIYUN_OSS_SIGNED_URL_EXPIRES`, expected to be 600 seconds.

## OSS Storage Design

Generated images upload to:

```text
oss://whalesing-web/aiImg/
```

Object keys follow:

```text
aiImg/users/{userId}/{channelSlug}/{modelSlug}/{YYYY-MM-DD}/{HHmmss}-{taskId}-{index}.{ext}
```

The server computes object keys. `channelSlug`, `modelSlug`, and extension values are sanitized. Supported image MIME types are PNG, JPEG, and WebP, with a safe fallback extension only when needed.

The storage interface should support:

- Upload bytes with content type.
- Download an upstream URL and upload to OSS.
- Convert base64 provider payloads to bytes and upload to OSS.
- Delete objects for history deletion if hard deletion is enabled.
- Generate a signed GET URL.

The MVP can hard-delete OSS objects when deleting history records. If deletion fails, the server should not leave database state inconsistent; either use a transaction around DB changes after object deletion or restore the soft-delete marker.

## Provider Adapter Design

The desktop TypeScript adapter behavior is the reference:

- OpenAI image generations/edits.
- Chat-completions-style image extraction.
- Gemini/Banana style inline image extraction.
- AihubMix/Doubao Seedream handling.
- Image-to-text via chat completions style calls.
- Upstream error formatting and API key redaction.

In the Go implementation, provider calls should be implemented behind a small `generation.Provider` boundary. Extraction behavior should be covered with Go tests using fixtures equivalent to the existing Vitest cases.

The first implementation does not need to support every possible provider response shape beyond the current desktop behavior, but it must preserve the known supported response patterns.

## Frontend Design

The Vite app has these routes:

- `/login`
- `/`
- `/settings`
- `/history`
- `/admin/users`

Route guards:

- Unauthenticated users go to `/login`.
- Authenticated non-admin users cannot access `/admin/users`.

Primary screens:

- Login page: username, password, submit, error display.
- Workspace page: channel/model/capability selection, prompt input, optional reference image, generation status, recent results.
- Settings page: CRUD channels and models; API key field only for create/replace.
- History page: filter and inspect own history records; reuse prompt and delete record.
- Admin users page: list users, create user, disable/enable, reset password.

The workspace can keep lightweight local preferences for channel/model/capability/size/count, but must never store API keys in localStorage.

Image display:

- History records contain file ids.
- The frontend requests signed URLs through `/api/files/{id}/url`.
- Signed URLs may be cached in component state until near expiry.

## Reuse From Desktop App

Read-only reference files:

- `D:\tool\app\page.tsx`
- `D:\tool\app\settings\page.tsx`
- `D:\tool\app\history\page.tsx`
- `D:\tool\app\components\ImageLightbox.tsx`
- `D:\tool\app\globals.css`
- `D:\tool\lib\model-helpers.ts`
- `D:\tool\lib\server\validation.ts`
- `D:\tool\lib\server\redaction.ts`
- `D:\tool\lib\server\format-upstream-error.ts`
- `D:\tool\lib\server\adapters.ts`
- `D:\tool\tests\*`

Reuse strategy:

- Port UI structure and CSS into `web/`.
- Port model helper behavior into TypeScript frontend utilities.
- Translate server validation, redaction, upstream error formatting, file-store behavior, history-store behavior, and adapters into Go packages.
- Recreate tests in Go for backend behavior and in frontend tests where browser behavior matters.

## Security Design

- Passwords are stored as hashes, never plaintext.
- API keys are encrypted before storage using `IMGTOOL_SECRET`.
- API keys are redacted from errors and logs.
- The frontend never receives raw stored API keys.
- OSS AccessKey values never leave the server.
- All protected APIs require a valid session.
- Disabled users cannot continue using old sessions.
- SQL queries for owned resources include `user_id`.
- File signed URL API verifies ownership before signing.
- Upload input images are limited to PNG, JPEG, and WebP and capped at 10 MB.
- Prompt length is capped at 4000 characters.

## Deployment And Startup

Development:

- Run the Go API server.
- Run the Vite dev server with API proxy to Go.

Production:

- Build the Vite frontend.
- Build the Go server.
- Run the Go server as the single web process.
- The Go server serves embedded static frontend assets and `/api/*`.

Suggested commands:

```bash
cd web
npm install
npm run build

cd ../server
go build -o ../dist/imgtool.exe ./cmd/imgtool
../dist/imgtool.exe
```

The server listens on `IMGTOOL_HTTP_ADDR`. Production can be supervised by Windows service tooling, PM2, NSSM, systemd, Docker, or another process manager. Docker is optional and not required for the first implementation.

## Testing Strategy

Backend tests:

- Password hashing verifies correct and incorrect passwords.
- Session creation, lookup, expiry, logout, and disabled-user rejection.
- Admin-only user management checks.
- Channel CRUD is scoped by `user_id`.
- API key encryption round-trips and rejects wrong secret.
- Generation requests reject raw API key fields and require channel ownership.
- History and task listing are scoped by `user_id`.
- Result file signing rejects non-owner access.
- OSS object keys match the required convention.
- Provider response extraction handles URL, markdown URL, base64 data URL, Gemini inline data, and text extraction.
- Upstream errors redact API keys.

Frontend tests:

- Login flow displays errors and stores no password locally.
- Settings page can create/edit/delete channels through API mocks.
- Generation page sends `channelId/modelId/capability`, not raw API key.
- History page requests signed URLs by file id.
- Admin page is hidden or blocked for non-admin users.

Manual smoke test:

1. Start app with `.env` configured.
2. Login as admin.
3. Create a normal user.
4. Login as normal user.
5. Add a channel with an API key.
6. Generate one image.
7. Confirm result metadata is in SQLite.
8. Confirm image object is uploaded under `aiImg/users/...`.
9. Confirm preview uses a signed URL.
10. Confirm another user cannot access the file URL.

## Implementation Decisions

- Use React Router for browser routes.
- Use Go standard library `net/http` routing.
- Use Argon2id for password hashing.
- Use `modernc.org/sqlite` unless implementation testing reveals a blocking compatibility issue.
- Embed `web/dist` into the production Go binary for single-process deployment.

## Acceptance Criteria

- The new app runs from `D:\code\aicode\ai\imgtool` without depending on Electron.
- The old `D:\tool` source remains unmodified.
- A user can log in, configure a private channel, generate an image, see it in history, and preview it through a signed OSS URL.
- Admin can create, disable, enable, and reset passwords for users.
- Normal users cannot view or mutate other users' channels, tasks, history, or files.
- No API response returns stored raw API keys.
- Generated files are stored in Aliyun OSS under the configured prefix and object key convention.
- Tests cover the security and ownership boundaries listed above.
