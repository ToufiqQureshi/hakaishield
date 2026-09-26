#!/usr/bin/env bash
# One-time provisioning for a fresh Ubuntu box. Run as root.
#
# Written against a DigitalOcean droplet (docs/DEPLOYMENT.md §3) but there is
# nothing provider-specific in it — it works on any Ubuntu host that gives you
# a plain public IP and port 443.
#
#   # First point the domain's DNS A record at this server.
#   bash deploy/setup.sh customer.example you@example.com
#
# It is deliberately not idempotent-magic: each step prints what it does so
# you can run them by hand instead if something looks wrong.
set -euo pipefail

DOMAIN="${1:?usage: setup.sh <domain> <email>}"
EMAIL="${2:?usage: setup.sh <domain> <email>}"

echo "==> Packages"
apt-get update -qq
# Ubuntu 24.04 ships the Compose CLI plugin as docker-compose-v2. The
# docker-compose-plugin name belongs to Docker's separate apt repository and
# is unavailable on a stock EC2/DigitalOcean Ubuntu image.
apt-get install -y -qq ufw certbot docker.io docker-compose-v2

echo "==> Firewall"
# Default-deny inbound. 80 stays open because certbot's HTTP-01 challenge
# needs it at renewal time, not just at issue time.
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp   comment 'ssh'
ufw allow 80/tcp   comment 'certbot http-01 renewal'
ufw allow 443/tcp  comment 'hakaishield'
ufw --force enable

echo "==> TLS certificate for ${DOMAIN}"
# DNS must already resolve to this box. certbot's standalone HTTP-01
# challenge cannot validate a host that still points elsewhere.
# --standalone binds :80 itself, so nothing else may be listening there.
# Without a certificate hakaishield cannot terminate TLS, which means no
# ClientHello, which means no JA4 - the product's core signal.
certbot certonly --standalone --non-interactive --agree-tos \
    -m "${EMAIL}" -d "${DOMAIN}"

echo "==> Install container-readable certificate copy"
install -m 0755 deploy/sync-certs.sh /usr/local/sbin/hakaishield-sync-certs
install -d -m 0755 /etc/hakaishield
printf '%s\n' "${DOMAIN}" > /etc/hakaishield/pilot-domain
chmod 0600 /etc/hakaishield/pilot-domain
/usr/local/sbin/hakaishield-sync-certs "/etc/letsencrypt/live/${DOMAIN}"

echo "==> Certificate renewal"
# certbot's packaged timer handles renewal. Copy the new key before restarting
# the non-root container; dry-run renewal alone does not exercise this hook.
mkdir -p /etc/letsencrypt/renewal-hooks/deploy
cat > /etc/letsencrypt/renewal-hooks/deploy/reload-hakaishield.sh <<'HOOK'
#!/usr/bin/env bash
set -eu
pilot_domain=$(cat /etc/hakaishield/pilot-domain)
if [ "${RENEWED_LINEAGE:-}" != "/etc/letsencrypt/live/${pilot_domain}" ]; then
    exit 0
fi
/usr/local/sbin/hakaishield-sync-certs "$RENEWED_LINEAGE"
if systemctl is-active --quiet hakaishield; then
    systemctl restart hakaishield
elif [ -f /opt/hakaishield/deploy/docker-compose.yml ]; then
    docker compose --env-file /opt/hakaishield/deploy/.env \
        -f /opt/hakaishield/deploy/docker-compose.yml restart hakaishield
fi
HOOK
chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-hakaishield.sh
systemctl enable --now certbot.timer

echo "==> Verifying Certbot renewal"
# This checks the ACME renewal path; the initial sync above tests the file copy.
certbot renew --dry-run

echo
echo "Done. Next:"
echo "  1. cp deploy/.env.example deploy/.env  and fill it in"
echo "  2. docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build"
echo "  3. Confirm ${DOMAIN}'s A record still points at this box"
echo
echo "It starts in shadow mode: it records decisions and acts on none."
echo "Watch it for a few days before switching HAKAISHIELD_MODE to enforce."
