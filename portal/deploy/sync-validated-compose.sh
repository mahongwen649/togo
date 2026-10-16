#!/bin/bash
# Apply the validated Compose source without rebuilding unchanged application images.
set -euo pipefail
umask 077
release=${1:?release directory required}
backup=${2:?backup directory required}
test -f "$release/portal/docker-compose.yml"
test ! -e "$backup"
mkdir -p "$backup"
cp /opt/portal/docker-compose.yml "$backup/docker-compose.yml"
cp /opt/portal/docker-compose.prod.yml "$backup/docker-compose.prod.yml"
docker inspect -f '{{.Id}} {{.Image}} {{.State.StartedAt}}' $(docker ps --format '{{.Names}}' | grep -v '^portal-portal-api-1$') > "$backup/protected-before.txt"
docker inspect -f '{{.Image}}' portal-portal-api-1 > "$backup/api-image-before.txt"
docker exec portal-portal-postgres-1 pg_dump -U portal -d portal -Fc > "$backup/portal.dump"
docker exec -i portal-portal-postgres-1 pg_restore -l < "$backup/portal.dump" > "$backup/restore-list.txt"
sha256sum "$backup/portal.dump" > "$backup/SHA256SUMS"
compose() { docker compose -p portal -f /opt/portal/docker-compose.yml -f /opt/portal/docker-compose.prod.yml --env-file /opt/portal/.env "$@"; }
rollback() {
  trap - ERR
  cp "$backup/docker-compose.yml" /opt/portal/docker-compose.yml
  compose up -d --no-deps --no-build --wait --wait-timeout 120 portal-api
  echo ROLLED_BACK >&2
  exit 1
}
trap rollback ERR
cp "$release/portal/docker-compose.yml" /opt/portal/docker-compose.yml
compose config --quiet
compose up -d --no-deps --no-build --wait --wait-timeout 120 portal-api
curl -fsS --max-time 30 http://127.0.0.1:3000/health
curl -fsS --max-time 30 https://togoapi.com/health
curl -fsS --max-time 30 https://img.togoapi.com/api/health
curl -fsS --max-time 30 https://api.togoapi.com/health
docker inspect -f '{{.Image}}' portal-portal-api-1 > "$backup/api-image-after.txt"
cmp "$backup/api-image-before.txt" "$backup/api-image-after.txt"
docker inspect -f '{{.Id}} {{.Image}} {{.State.StartedAt}}' $(docker ps --format '{{.Names}}' | grep -v '^portal-portal-api-1$') > "$backup/protected-after.txt"
cmp "$backup/protected-before.txt" "$backup/protected-after.txt"
echo DEPLOY_OK
