# Imgtool Web Configuration

This project will be built as a private multi-account web app, not a full SaaS product.

## Confirmed Product Boundary

- Deployment mode: B+ simple account-isolated web version.
- There is one built-in `admin` account.
- `admin` can create accounts, disable accounts, and reset passwords.
- Normal users cannot self-register.
- Core owns user API keys and model groups; Imgtool does not ask users to configure them.
- Each user can only access their own tasks, history, and generated images.
- Login uses username and password.
- The app uses minimal server-side login checks with an HttpOnly cookie and server session validation.
- Passwords are stored as hashes, not plaintext.
- Imgtool receives the selected user's Core image key server-to-server through Portal and never exposes it to the browser.

## Database And Storage

- SQLite stores users, sessions, channels, tasks, history records, and generated image metadata.
- Generated image files are not stored in SQLite.
- Generated image files are uploaded to Aliyun OSS.
- File metadata stores the OSS bucket, object key, MIME type, size, owner user id, and history record id.

## Tencent COS Target

Generated images can be uploaded to Tencent COS. Production should use the private bucket:

```text
cos://togoapi-img-1385779910/aiImg/
```

Use this environment configuration:

```env
IMGTOOL_STORAGE_PROVIDER=tencent-cos
TENCENT_COS_BUCKET=togoapi-img-1385779910
TENCENT_COS_REGION=ap-guangzhou
TENCENT_COS_ENDPOINT=https://togoapi-img-1385779910.cos.ap-guangzhou.myqcloud.com
TENCENT_COS_PREFIX=aiImg
TENCENT_COS_SIGNED_URL_EXPIRES=600
```

The SecretId and SecretKey must be stored only in the server `.env`:

```env
TENCENT_COS_SECRET_ID=...
TENCENT_COS_SECRET_KEY=...
```

## Aliyun OSS Target

Generated images must be uploaded under:

```text
oss://whalesing-web/aiImg/
```

Use this environment configuration:

```env
IMGTOOL_STORAGE_PROVIDER=aliyun-oss
ALIYUN_OSS_BUCKET=whalesing-web
ALIYUN_OSS_ENDPOINT=https://oss-cn-guangzhou.aliyuncs.com
ALIYUN_OSS_PREFIX=aiImg
ALIYUN_OSS_SIGNED_URL_EXPIRES=600
```

The AccessKeyId and AccessKeySecret are available locally in `oss.txt`, but they must not be committed. Move them into `.env` when implementing:

```env
ALIYUN_OSS_ACCESS_KEY_ID=...
ALIYUN_OSS_ACCESS_KEY_SECRET=...
```

## Object Key Convention

Store generated images below the configured prefix, grouped by user:

```text
aiImg/users/{userId}/{channelSlug}/{modelSlug}/{YYYY-MM-DD}/{HHmmss}-{taskId}-{index}.{ext}
```

Example:

```text
aiImg/users/usr_123/openai/gpt-image-1/2026-07-07/153022-taskid-0.png
```

## Access Policy

- The OSS bucket should be private.
- The app should not expose OSS AccessKeys to the browser.
- The backend should generate short-lived signed URLs for image preview/download.
- Default signed URL expiry: 600 seconds.
- All file preview APIs must verify that the current logged-in user owns the file metadata before returning a signed URL.

## Main Site Identity

Imgtool is entered from the main Sub2API site and does not maintain a separate user-facing account system.

Configure the same shared SSO secret in both services:

```env
# sub2api
IMGTOOL_BASE_URL=https://img.example.com
IMGTOOL_SSO_SECRET=replace-with-same-32-byte-secret-as-imgtool

# imgtool
IMGTOOL_SSO_SECRET=replace-with-same-32-byte-secret-as-sub2api
IMGTOOL_SSO_ISSUER=sub2api
IMGTOOL_SSO_AUDIENCE=imgtool
```

Flow:

```text
Sub2API menu -> POST /api/v1/integrations/imgtool/sso-url -> https://img.example.com/sso?ticket=...
Imgtool validates the short-lived ticket, creates its own HttpOnly session cookie, then redirects to /.
```

When a browser reaches Imgtool without a valid session, the frontend shows a "please enter from the main site" message instead of an account/password login form.

## Image Generation Providers

The image workspace has two providers: OpenAI and Grok. On SSO entry, Portal checks for
an active user key in Core group `15` (`gpt-image-2【5分钱一张】`) and group `33`
(`Grok Heavy`), creating the missing key in the background. Imgtool then queries the
models visible to each key and keeps only image-generation models. Users only choose the
provider and model; no API key, Base URL, or manual channel setup is required.

The workspace supports:

- text-to-image;
- image-to-image with one reference image.

Grok Imagine models additionally support:

- text-to-image with `aspect_ratio`, `resolution` (`1k` or `2k`), and `quality` (`low` or `medium`);
- image-to-image with one reference image, `aspect_ratio` (including `auto`) and `resolution`.

Grok requests use `response_format=b64_json` so Imgtool does not need to fetch temporary
`imgen.x.ai` URLs. Provider-prefixed IDs such as `xai/grok-imagine-image-2.0` are also
recognized by the workspace.

For production deployments on the same host as Sub2API Core, set
`IMGTOOL_INTERNAL_CORE_BASE_URL=http://127.0.0.1:8080`. Imgtool will use this address for
Core requests, avoiding the CDN timeout on long-running image jobs.

## Deletion Policy

Default recommendation:

- Deleting a history record should remove or soft-delete the database record.
- Whether OSS objects are deleted immediately can be implemented as a setting.
- First implementation may support hard deletion of both DB metadata and OSS objects to keep storage usage predictable.

## Existing Desktop App Source

The previous desktop app is located at:

```text
D:\tool
```

It is a Next.js app wrapped by Electron. The Electron layer mostly starts a local Next server and opens a BrowserWindow. The migration should reuse the Next frontend and backend logic where practical, while removing Electron and changing storage/auth boundaries.
