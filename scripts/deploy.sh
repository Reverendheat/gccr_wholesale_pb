#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# deploy.sh — pull latest image and restart the stack
#
# Usage (from the server):
#   cd /opt/gccr_wholesale_pb && sudo bash scripts/deploy.sh
# -----------------------------------------------------------------------------
set -euo pipefail

info() { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok()   { echo -e "\033[1;32m[ OK ]\033[0m  $*"; }

info "Pulling latest config from git…"
git pull

info "Pulling latest Docker image…"
docker compose pull

info "Restarting stack…"
docker compose up -d --remove-orphans

info "Cleaning up old images…"
docker image prune -f

ok "Deploy complete"
docker compose ps
