#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# setup-ubuntu.sh — one-shot provisioning for Ubuntu 22.04
#
# Requires the EC2 instance to have an IAM role with ssm:GetParameter access
# to the /wholesale/* parameter path.
#
# Usage:
#   sudo bash setup-ubuntu.sh
# -----------------------------------------------------------------------------
set -euo pipefail

APP_DIR="/opt/gccr_wholesale_pb"
SSM_PREFIX="/wholesale"

# ── helpers ──────────────────────────────────────────────────────────────────
info() { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok()   { echo -e "\033[1;32m[ OK ]\033[0m  $*"; }
die()  { echo -e "\033[1;31m[ERR ]\033[0m  $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "Please run as root: sudo bash $0"

# When invoked via sudo the AWS CLI runs as root but credentials live in the
# invoking user's home directory.  Point the SDK at the right files.
SUDO_USER_HOME=$(getent passwd "${SUDO_USER:-ubuntu}" | cut -d: -f6)
export AWS_CONFIG_FILE="${SUDO_USER_HOME}/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="${SUDO_USER_HOME}/.aws/credentials"

ssm_get() {
  # $1 = parameter name (without prefix), $2 = "secure" to decrypt
  local name="${SSM_PREFIX}/$1"
  local decrypt_flag=""
  [[ "${2:-}" == "secure" ]] && decrypt_flag="--with-decryption"
  aws ssm get-parameter --name "$name" $decrypt_flag \
    --query "Parameter.Value" --output text
}

# ── 1. system packages ───────────────────────────────────────────────────────
info "Updating system packages…"
apt-get update -qq
apt-get upgrade -y -qq
apt-get install -y -qq curl git ufw ca-certificates gnupg unzip

# ── 2. AWS CLI ───────────────────────────────────────────────────────────────
if ! command -v aws &>/dev/null; then
  info "Installing AWS CLI v2…"
  curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscliv2.zip
  unzip -q /tmp/awscliv2.zip -d /tmp/awscli
  /tmp/awscli/aws/install
  rm -rf /tmp/awscliv2.zip /tmp/awscli
  ok "AWS CLI installed: $(aws --version)"
else
  ok "AWS CLI already present: $(aws --version)"
fi

# ── 3. Docker Engine ─────────────────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
  info "Installing Docker Engine…"
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io \
    docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
  ok "Docker installed: $(docker --version)"
else
  ok "Docker already installed: $(docker --version)"
fi

# ── 4. swap (small amount for runtime safety) ────────────────────────────────
SWAP_FILE="/swapfile"
if swapon --show | grep -q "$SWAP_FILE"; then
  ok "Swap already active"
else
  info "Creating 1GB swap file…"
  fallocate -l 1G "$SWAP_FILE"
  chmod 600 "$SWAP_FILE"
  mkswap "$SWAP_FILE"
  swapon "$SWAP_FILE"
  grep -q "$SWAP_FILE" /etc/fstab || echo "$SWAP_FILE none swap sw 0 0" >> /etc/fstab
  ok "Swap enabled (1GB)"
fi

# ── 5. fetch secrets from SSM ────────────────────────────────────────────────
info "Fetching parameters from SSM (${SSM_PREFIX}/*)…"
GITHUB_TOKEN=$(ssm_get "github_token" secure)
ENV_FILE_CONTENT=$(ssm_get "env")
ADMIN_EMAIL=$(ssm_get "admin_email")
ADMIN_PASSWORD=$(ssm_get "admin_password" secure)
ok "SSM parameters loaded"

# ── 6. clone / update repo (for compose + Caddyfile only, no build needed) ──
REPO_URL="https://x-access-token:${GITHUB_TOKEN}@github.com/Reverendheat/gccr_wholesale_pb.git"

if [[ -d "$APP_DIR/.git" ]]; then
  info "Repo already present — pulling latest…"
  git -C "$APP_DIR" remote set-url origin "$REPO_URL"
  git -C "$APP_DIR" pull
else
  info "Cloning repo to $APP_DIR…"
  git clone "$REPO_URL" "$APP_DIR"
fi

cd "$APP_DIR"

# ── 7. write .env ────────────────────────────────────────────────────────────
info "Writing .env from SSM…"
printf '%s\n' "$ENV_FILE_CONTENT" > .env
ok ".env written"

# ── 8. authenticate with GHCR & pull + start ─────────────────────────────────
info "Logging in to GitHub Container Registry…"
echo "${GITHUB_TOKEN}" | docker login ghcr.io -u Reverendheat --password-stdin
ok "GHCR login successful"

info "Pulling latest images and starting stack…"
docker compose pull
docker compose up -d
ok "Stack is up"

# ── 9. create PocketBase superuser ───────────────────────────────────────────
info "Waiting for app to be healthy…"
for i in $(seq 1 30); do
  if docker compose exec -T app wget -qO- http://localhost:8090/api/health &>/dev/null; then
    ok "App is healthy"
    break
  fi
  [[ $i -eq 30 ]] && die "App did not become healthy in time"
  sleep 2
done

info "Creating PocketBase superuser…"
docker compose exec -T app ./server superuser upsert "$ADMIN_EMAIL" "$ADMIN_PASSWORD" \
  --dir /app/pb_data \
  && ok "Superuser created: $ADMIN_EMAIL" \
  || ok "Superuser already exists (upsert skipped)"

# ── done ─────────────────────────────────────────────────────────────────────
echo ""
echo "  ✓ Deployment complete"
echo ""
echo "  Useful commands (run from $APP_DIR):"
echo "    docker compose logs -f          # tail logs"
echo "    docker compose ps               # container status"
echo "    docker compose pull && docker compose up -d  # redeploy after a push"
echo "    docker compose down             # stop the stack"
echo ""
