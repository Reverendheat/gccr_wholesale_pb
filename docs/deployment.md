# Deployment and Operations

## Overview

Production runs two Docker Compose services:

- `app`: Go/PocketBase server plus built React frontend
- `caddy`: public HTTP/HTTPS reverse proxy and automatic TLS

PocketBase data lives in the persistent `pb_data` Docker volume. Caddy certificates and state live in separate `caddy_data` and `caddy_config` volumes.

A push to `main` triggers `.github/workflows/build-push.yml`. GitHub Actions builds frontend and server, then publishes these image tags to GHCR:

- `ghcr.io/reverendheat/gccr_wholesale_pb:latest`
- `ghcr.io/reverendheat/gccr_wholesale_pb:sha-<7-character-commit>`

Deployments are completed manually on server with `scripts/deploy.sh`.

## First-deployment prerequisites

### Server and network

- Ubuntu 22.04 host
- DNS `A`/`AAAA` record pointing production domain to host
- Inbound TCP 80 and 443; UDP 443 optional for HTTP/3
- SSH or AWS Session Manager access
- SSH restricted to trusted IP addresses
- EC2 IAM role allowed to call `ssm:GetParameter`; include `kms:Decrypt` for SecureString parameters

`setup-ubuntu.sh` installs Docker and AWS CLI. Firewall commands in script are currently commented out, so enforce network restrictions with EC2 security groups or configure UFW separately.

### Caddy configuration

Update domain and PocketBase admin allowlist in `Caddyfile` before deployment.

> Security warning: repository currently uses `remote_ip 0.0.0.0/0`, which allows every IP to reach PocketBase admin panel. Replace it with trusted public IP/CIDR entries.

Example:

```caddyfile
wholesale.example.com {
    @admin path /_/*
    handle @admin {
        @allowed remote_ip 203.0.113.10/32
        handle @allowed {
            reverse_proxy app:8090
        }
        respond 403
    }

    reverse_proxy app:8090
}
```

### AWS Systems Manager Parameter Store

Provision these parameters under `/wholesale`:

| Parameter | Suggested type | Purpose |
|---|---|---|
| `/wholesale/github_token` | SecureString | Private GitHub repository and GHCR access |
| `/wholesale/env` | String | Complete production `.env` contents |
| `/wholesale/admin_email` | String | Initial PocketBase superuser email |
| `/wholesale/admin_password` | SecureString | Initial PocketBase superuser password |

Current setup script fetches `/wholesale/env` without `--with-decryption`, so it must currently be a String. It still contains secrets: tightly restrict SSM and IAM access. Do not commit production `.env`.

GitHub token needs read access to repository contents and packages.

### Production environment

Store following values in `/wholesale/env`. Square token, location, and category must all come from same Square environment.

```dotenv
SQUARE_ACCESS_TOKEN=...
SQUARE_LOCATION_ID=...
SQUARE_WHOLESALE_CATEGORY_ID=...
SQUARE_SANDBOX=false

SQUARE_WEBHOOK_URL=https://wholesale.example.com/api/webhooks/square
SQUARE_WEBHOOK_SIGNATURE_KEY=...

PB_APP_NAME=GCCR Wholesale
PB_APP_URL=https://wholesale.example.com
PB_SENDER_NAME=GCCR Wholesale
PB_SENDER_ADDRESS=no-reply@example.com

PB_SMTP_ENABLED=true
PB_SMTP_HOST=...
PB_SMTP_PORT=587
PB_SMTP_USERNAME=...
PB_SMTP_PASSWORD=...
PB_SMTP_AUTH_METHOD=PLAIN
PB_SMTP_TLS=false
```

`SQUARE_SANDBOX=true` uses Square sandbox. Sandbox and production use different access tokens, location IDs, customer records, category IDs, and catalog records.

SMTP is required for:

- Staff one-time sign-in codes
- Customer one-time sign-in codes
- Customer welcome emails

### Square webhook

In Square Developer Dashboard:

1. Register `SQUARE_WEBHOOK_URL`.
2. Subscribe to `invoice.payment_made`, `invoice.canceled`, and `invoice.refunded`.
3. Put webhook signature key in `SQUARE_WEBHOOK_SIGNATURE_KEY`.
4. Ensure webhook environment matches `SQUARE_SANDBOX`.

GCCR Wholesale owns submitted order contents and fulfillment workflow. Submission locks catalog prices locally; Square order and invoice are created together when staff sends invoice. Square owns billing state: payment marks local order `paid`, cancellation marks it `cancelled`, and refunds require `needs_review`. Do not edit Square order line items directly. Invoice polling runs every 15 minutes as fallback for missed payment or cancellation webhooks.

## Initial provisioning

Copy `scripts/setup-ubuntu.sh` to host, then run it as root:

```bash
sudo bash setup-ubuntu.sh
```

Script:

1. Installs system packages, AWS CLI, and Docker.
2. Creates swap.
3. Fetches SSM parameters.
4. Clones repository to `/opt/gccr_wholesale_pb`.
5. Writes `/opt/gccr_wholesale_pb/.env`.
6. Authenticates with GHCR.
7. Pulls and starts Compose stack.
8. Waits for PocketBase health endpoint.
9. Creates or updates PocketBase superuser from SSM credentials.

Verify:

```bash
cd /opt/gccr_wholesale_pb
sudo docker compose ps
sudo docker compose logs --tail=100 app
curl -fsS https://wholesale.example.com/api/health
```

Open application at `https://wholesale.example.com/`. From allowed admin IP, open PocketBase at `https://wholesale.example.com/_/`.

## Deploying application updates

1. Commit changes and push to `main`.
2. Wait for GitHub Action **Build & Push Docker Image** to finish successfully.
3. Connect to host and run:

```bash
cd /opt/gccr_wholesale_pb
sudo bash scripts/deploy.sh
```

4. Verify:

```bash
sudo docker compose ps
sudo docker compose logs --tail=100 app
curl -fsS https://wholesale.example.com/api/health
```

`deploy.sh` pulls Git configuration and latest image, recreates changed containers, and prunes old images. No machine reboot is required.

## Updating environment variables

`deploy.sh` does **not** refresh `.env` from `/wholesale/env`.

For durable configuration, update both SSM `/wholesale/env` and server `/opt/gccr_wholesale_pb/.env`. Then recreate app container:

```bash
cd /opt/gccr_wholesale_pb
sudo nano .env
sudo docker compose up -d --force-recreate app
sudo docker compose logs --tail=100 app
```

`docker compose restart app` does not reload `.env`; environment is fixed when container is created.

Required startup values:

- `SQUARE_ACCESS_TOKEN`
- `SQUARE_LOCATION_ID`
- `SQUARE_WHOLESALE_CATEGORY_ID`

Missing required value causes app restart loop with error in logs.

## Service operations

```bash
cd /opt/gccr_wholesale_pb

# Status
sudo docker compose ps

# Logs
sudo docker compose logs -f app
sudo docker compose logs -f caddy

# Restart existing containers; does not pull code or reload .env
sudo docker compose restart

# Recreate app and reload .env
sudo docker compose up -d --force-recreate app

# Stop stack without deleting volumes
sudo docker compose down

# Start stack
sudo docker compose up -d
```

Never use `docker compose down -v` unless intentionally deleting PocketBase data and Caddy state.

## Database migrations

Go migrations are compiled into server binary. PocketBase applies new migrations automatically before HTTP server starts and records applied filenames in database.

Rules:

- Never edit migration already deployed.
- Add new timestamp-named migration for every schema change.
- Test against both existing database and empty data directory.
- Back up `pb_data` before production schema migration.

Migration failure prevents app from starting. Inspect app logs for exact migration filename and error.

## Backing up PocketBase data

Create backup directory, stop app for consistent SQLite snapshot, archive only `pb_data`, then restart:

```bash
cd /opt/gccr_wholesale_pb
mkdir -p backups
sudo docker compose stop app

PB_VOLUME=$(sudo docker volume ls -q \
  --filter label=com.docker.compose.project=gccr_wholesale_pb \
  --filter label=com.docker.compose.volume=pb_data)

sudo docker run --rm \
  -v "$PB_VOLUME:/data:ro" \
  -v "$PWD/backups:/backup" \
  alpine sh -c 'tar czf /backup/pb_data-$(date +%Y%m%d-%H%M%S).tar.gz -C /data .'

sudo docker compose up -d app
```

Copy backups off host. Test restore process before relying on backups.

## Resetting PocketBase superuser credentials

Update `/wholesale/admin_email` and `/wholesale/admin_password`, then apply values to PocketBase:

```bash
cd /opt/gccr_wholesale_pb

ADMIN_EMAIL=$(aws ssm get-parameter \
  --name /wholesale/admin_email \
  --query Parameter.Value --output text)

ADMIN_PASSWORD=$(aws ssm get-parameter \
  --name /wholesale/admin_password \
  --with-decryption \
  --query Parameter.Value --output text)

sudo docker compose exec -T app ./server superuser upsert \
  "$ADMIN_EMAIL" "$ADMIN_PASSWORD" \
  --dir /app/pb_data

unset ADMIN_EMAIL ADMIN_PASSWORD
```

Regular deployment does not change superuser credentials.

## Troubleshooting

### App continuously restarts

```bash
sudo docker compose logs --tail=200 app
```

Common causes:

- Missing required Square environment value
- Invalid migration ordering or migration failure
- Invalid PocketBase/SMTP setting
- Corrupt or inaccessible `pb_data`

### New image is not running

Confirm GitHub Action completed before deployment, then:

```bash
sudo docker compose pull app
sudo docker compose up -d --force-recreate app
```

### Environment change has no effect

Recreate container; restart is insufficient:

```bash
sudo docker compose up -d --force-recreate app
```

### Customer invitation or OTP email fails

Check:

- `PB_APP_URL`
- Sender address/name
- `PB_SMTP_*`
- App logs
- SMTP provider delivery logs and spam filtering

Customer record may still be created when welcome email delivery fails.

### Catalog is empty

Check Square environment and verify:

- Access token is valid
- `SQUARE_SANDBOX` selects expected environment
- `SQUARE_WHOLESALE_CATEGORY_ID` belongs to same environment
- Items are assigned to category
- Items have fixed-price variations
