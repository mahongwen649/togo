#!/bin/sh
set -eu

umask 077
printf '%s\n' "$CERTBOT_DOMAIN" > /var/lib/letsencrypt/imgtool-acme-domain
printf '%s\n' "$CERTBOT_VALIDATION" > /var/lib/letsencrypt/imgtool-acme-validation

while [ ! -f /var/lib/letsencrypt/imgtool-acme-ready ]; do
    sleep 2
done
