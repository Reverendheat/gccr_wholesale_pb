#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# deploy.sh — pull latest image and restart the stack
#
# Usage (from the server):
#   cd /opt/gccr_wholesale_pb && sudo bash scripts/deploy.sh
# -----------------------------------------------------------------------------
set -euo pipefail

SSM_PREFIX="/wholesale"

info() { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok()   { echo -e "\033[1;32m[ OK ]\033[0m  $*"; }
die()  { echo -e "\033[1;31m[ERR ]\033[0m  $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "Please run as root: sudo bash $0"

SUDO_USER_HOME=$(getent passwd "${SUDO_USER:-ubuntu}" | cut -d: -f6)
export AWS_CONFIG_FILE="${SUDO_USER_HOME}/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="${SUDO_USER_HOME}/.aws/credentials"

info "Fetching GitHub token from SSM…"
GITHUB_TOKEN=$(aws ssm get-parameter --name "${SSM_PREFIX}/github_token" --with-decryption \
  --query "Parameter.Value" --output text)

info "Logging in to GHCR…"
echo "${GITHUB_TOKEN}" | docker login ghcr.io -u Reverendheat --password-stdin

info "Pulling latest config from git…"
git remote set-url origin "https://x-access-token:${GITHUB_TOKEN}@github.com/Reverendheat/gccr_wholesale_pb.git"
git pull

info "Pulling latest Docker image…"
docker compose pull

info "Restarting stack…"
docker compose up -d --remove-orphans

info "Cleaning up old images…"
docker image prune -f

ok "Deploy complete"
docker compose ps
