# Portal usage-record deployment — 2026-11-03

## Release

- Application commit: c02088219d33d21671161c39b8ff8d9af8930e3e, pushed to origin/main.
- Target: Tokyo, /opt/portal, existing production Compose configuration.
- API started: 2026-11-03 12:06:03 Asia/Shanghai.
- Final Web started: 2026-11-03 12:10:37 Asia/Shanghai.
- Final verification: 2026-11-03 12:15:42 Asia/Shanghai.
- Only Portal API/Web were recreated. Core and all eleven other containers retained their identities, images, and start times. No migrations were added.

The admin usage page now displays output TPS as output tokens divided by full request duration (including first-token wait), rounded to one decimal tok/s. It adds account search/selection, Live request filtering, native-compaction filtering, upstream-model audit filtering, error-phase filtering, and corresponding export fields. Account search is administrator-only and returns sanitized account IDs/names through Core's official API.

## Validation

- 57 focused frontend tests passed across seven files; type checking, scoped ESLint, and production build passed.
- Local Go tests and the staged Linux backend test/build passed. PostgreSQL integration tests requiring PORTAL_RENTAL_TEST_DATABASE_URL were skipped because the test database was not configured.
- Public Portal, Core, and Imgtool health checks and the admin usage route returned HTTP 200. Anonymous account search returned HTTP 401.
- Public entry, locale, usage, and vendor JS/CSS SHA-256 hashes matched the artifact. Twenty deployed changed source files matched the application commit's manifest.
- Both Portal containers are healthy with zero restarts; startup logs have no ERROR, FATAL, or panic matches during the observed window.
- Existing authenticated admin browser: account search returned matching name/ID choices; selecting weilaiapi restricted all 50 displayed rows to that account; reset cleared the filter. TPS rendered, including 132 output tokens / 14.86 seconds = 8.9 tok/s. Native-compaction filtering returned ten records. Live, model-consistency, and error-phase options were available. No browser console errors were captured.
- No paid synthetic inference request was sent. This is a deployment smoke test, not proof that existing upstream account failures are resolved.

## Artifacts and recovery

- Release directory: /opt/releases/portal-usage-parity-20261103/
- Backup directory: /opt/backups/portal-usage-parity-20261103/
- PostgreSQL custom backup: 3,153,775 bytes; restore listing readable; backup SHA-256 checks passed. A full database restoration was not rehearsed.
- Backup also includes previous source, Compose files, protected environment file, and container identities. Secrets remain only on the server.
- API image: portal-portal-api:usage-parity-20261103, sha256:902d83806461a5f79a62dc893c558e252c1aff08f53a5bc932fb83119c8b083d.
- Web image: portal-portal-web:usage-parity-20261103, sha256:2a11011cca86ba7d0c57cf03e0217e72264a0cc09e5dce74b058594f6b7183a4.
- Initial release archive SHA-256: 9009a7824edf1bd5319a74b7d3774a0cc500271241e098cc68f335017c79e4b4.
- Final frontend archive SHA-256: 1cc380822f95e914c76adc69b5a1252c70d301ee03a8387b3d5369e0ecd9e6b4.

Production source comparison found only extra trailing blank lines in accountBusiness.ts and AccountUsageCell.vue outside the changed files; those production files were preserved. Documentation/test-only baseline differences were also preserved.

The initial archive extraction inherited restrictive directory modes, causing Nginx to return 404 for new assets after the first cutover. The running container's read/traverse permissions were repaired, then fixed permanently in the Web image with chmod -R a+rX /usr/share/nginx/html. Because the CDN had cached the transient 404 responses, the same application source was rebuilt with pnpm exec vite build --assetsDir assets-usage-parity-20261103 and deployed under fresh asset URLs. The final artifact and public-resource hash checks passed. Preserve the explicit readable-directory step in future archive-based releases, and check assets as the nginx user before activation.

Image rollback, if required:

    cd /opt/portal
    sudo -n docker tag portal-portal-api:before-usage-parity-20261103 portal-portal-api:latest
    sudo -n docker tag portal-portal-web:before-usage-parity-20261103 portal-portal-web:latest
    sudo -n docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env up -d --no-deps --no-build --wait --wait-timeout 120 portal-api portal-web

Recheck health and the usage page afterward. No database restore is required for this release. For source parity after a manual image rollback, restore only the files in the release manifest from the backed-up source archive and remove only newly introduced release files whose contents still match the manifest; preserve any subsequent changes. Do not run a broad Compose down or restart Core.
