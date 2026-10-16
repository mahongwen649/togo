#!/usr/bin/env bash
set -euo pipefail
umask 077
release=/opt/portal-releases/account-prompt-20261005
backup=/opt/backups/portal-account-prompt-20261005
image=portal-portal-api:account-prompt-20261005
files=(backend/internal/coreclient/account_prompt.go backend/internal/coreclient/account_prompt_test.go backend/internal/coreclient/account_prompt_live_test.go backend/internal/httpapi/account_checks.go backend/internal/httpapi/account_checks_test.go backend/internal/httpapi/account_check_retry_test.go backend/internal/httpapi/account_check_history_test.go docs/admin-account-checks.md)
protected=(sub2api sub2api-postgres sub2api-redis newsub-app newsub-postgres newsub-redis portal-portal-postgres-1 portal-portal-web-1)
case "${1:-}" in
prepare)
  test ! -e "$release"
  printf '%s  %s\n' "$2" /tmp/portal-account-prompt-20261005.tar | sha256sum -c -
  mkdir -p "$release" "$backup"
  cd /opt/portal
  tar --exclude=.env --exclude='*.log' -cf "$backup/source.tar" backend docs Dockerfile.api
  tar -xf "$backup/source.tar" -C "$release"
  tar -xf /tmp/portal-account-prompt-20261005.tar -C "$release"
  sha256sum .env docker-compose.yml docker-compose.prod.yml > "$backup/protected.sha256"
  docker inspect --format '{{.Id}} {{.Image}} {{.State.StartedAt}}' "${protected[@]}" > "$backup/protected-containers.txt"
  old_image=$(docker inspect --format '{{.Image}}' portal-portal-api-1)
  docker tag "$old_image" portal-portal-api:rollback-account-prompt-20261005
  printf '%s\n' "$old_image" > "$backup/old-image.txt"
  cd "$release"
  sha256sum "${files[@]}" > "$backup/release.sha256"
  docker build -f deploy/Dockerfile.account-prompt-validation -t portal-account-prompt-validation:20261005 . > "$backup/validation-build.log" 2>&1
  docker build -f Dockerfile.api --build-arg GOLANG_IMAGE=golang:1.27.0-alpine -t "$image" . > "$backup/build.log" 2>&1
  touch "$backup/build-passed"
  echo "PORTAL_BUILD_OK $image"
  ;;
rebuild)
  test -d "$release"
  test ! -f "$backup/activated"
  printf '%s  %s\n' "$2" /tmp/portal-account-prompt-20261005.tar | sha256sum -c -
  tar -xf /tmp/portal-account-prompt-20261005.tar -C "$release"
  cd "$release"
  sha256sum "${files[@]}" > "$backup/release.sha256"
  docker build -f deploy/Dockerfile.account-prompt-validation -t portal-account-prompt-validation:20261005 . > "$backup/validation-build.log" 2>&1
  docker build -f Dockerfile.api --build-arg GOLANG_IMAGE=golang:1.27.0-alpine -t "$image" . > "$backup/build.log" 2>&1
  touch "$backup/build-passed"
  echo "PORTAL_BUILD_OK $image"
  ;;
activate)
  test -f "$backup/build-passed"
  active=$(docker exec portal-portal-postgres-1 sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT count(*) FROM portal_admin_account_checks WHERE state = '\''running'\''"')
  test "$active" = 0 || { echo 'Wait for running Portal account checks to finish before activation' >&2; exit 1; }
  cd /opt/portal
  sha256sum -c "$backup/protected.sha256"
  docker inspect --format '{{.Id}} {{.Image}} {{.State.StartedAt}}' "${protected[@]}" > "$backup/protected-before-activation.txt"
  cmp "$backup/protected-containers.txt" "$backup/protected-before-activation.txt"
  rollback() {
    trap - ERR
    docker tag portal-portal-api:rollback-account-prompt-20261005 portal-portal-api:latest
    tar -xf "$backup/source.tar" -C /opt/portal
    rm -f /opt/portal/backend/internal/coreclient/account_prompt.go /opt/portal/backend/internal/coreclient/account_prompt_test.go /opt/portal/backend/internal/coreclient/account_prompt_live_test.go
    docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --no-deps --no-build portal-api
    echo "Portal API rollback completed" >&2
    exit 1
  }
  trap rollback ERR
  for file in "${files[@]}"; do cp "$release/$file" "$file"; done
  sha256sum -c "$backup/release.sha256"
  docker tag "$image" portal-portal-api:latest
  docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --no-deps --no-build --wait --wait-timeout 90 portal-api
  curl -fsS --max-time 15 http://127.0.0.1:3000/health
  sha256sum -c "$backup/protected.sha256"
  docker inspect --format '{{.Id}} {{.Image}} {{.State.StartedAt}}' "${protected[@]}" > "$backup/protected-after-activation.txt"
  cmp "$backup/protected-containers.txt" "$backup/protected-after-activation.txt"
  docker inspect --format '{{.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}' portal-portal-api-1
  touch "$backup/activated"
  echo "PORTAL_ACTIVATE_OK Core and other containers unchanged"
  ;;
*) exit 2 ;;
esac
