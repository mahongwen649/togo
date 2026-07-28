#!/usr/bin/env bash
set -euo pipefail

site=/etc/nginx/sites-available/sub2api
locations=/etc/nginx/snippets/gift.locations.conf
rate_limits=/etc/nginx/conf.d/gift-rate-limit.conf
backup="${site}.bak-gift-$(date +%Y%m%d-%H%M%S)"

sudo cp "$site" "$backup"
sudo install -o root -g root -m 644 /tmp/nginx-gift.locations.conf "$locations"
sudo install -o root -g root -m 644 /tmp/nginx-gift-rate-limit.conf "$rate_limits"

if ! sudo grep -qF 'include /etc/nginx/snippets/gift.locations.conf;' "$site"; then
  awk '
    { print }
    /server_name togoapi.com;/ { target = 1 }
    target && /ssl_protocols TLSv1.2 TLSv1.3;/ {
      print ""
      print "    include /etc/nginx/snippets/gift.locations.conf;"
      target = 0
    }
  ' "$site" > /tmp/sub2api.next
  sudo install -o root -g root -m 644 /tmp/sub2api.next "$site"
fi

if ! sudo nginx -t; then
  sudo cp "$backup" "$site"
  sudo rm -f "$locations" "$rate_limits"
  sudo nginx -t
  exit 1
fi

sudo systemctl reload nginx
echo "backup=$backup"
sudo nginx -t
sudo systemctl is-active nginx
