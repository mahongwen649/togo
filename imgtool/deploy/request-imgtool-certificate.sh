#!/bin/sh
set -eu

rm -f \
    /var/lib/letsencrypt/imgtool-acme-domain \
    /var/lib/letsencrypt/imgtool-acme-validation \
    /var/lib/letsencrypt/imgtool-acme-ready \
    /var/log/certbot-imgtool.log

nohup certbot certonly \
    --manual \
    --preferred-challenges dns \
    --manual-auth-hook /usr/local/sbin/imgtool-certbot-auth \
    --manual-cleanup-hook /usr/local/sbin/imgtool-certbot-cleanup \
    --non-interactive \
    --agree-tos \
    --register-unsafely-without-email \
    --cert-name img.togoapi.com \
    -d img.togoapi.com \
    > /var/log/certbot-imgtool.log 2>&1 &

echo $! > /run/certbot-imgtool.pid

i=0
while [ "$i" -lt 15 ]; do
    if [ -s /var/lib/letsencrypt/imgtool-acme-validation ]; then
        cat /var/lib/letsencrypt/imgtool-acme-domain
        cat /var/lib/letsencrypt/imgtool-acme-validation
        exit 0
    fi
    i=$((i + 1))
    sleep 1
done

cat /var/log/certbot-imgtool.log >&2
exit 1
