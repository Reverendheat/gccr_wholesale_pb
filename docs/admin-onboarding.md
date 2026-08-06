# Administrator Onboarding

## Administrator versus staff

PocketBase administrator and application staff are separate identities:

- **PocketBase administrator** signs into `https://<domain>/_/` using email/password and has unrestricted database/configuration access.
- **Staff** signs into main application using one-time email code and has application workflows only.

Create a separate `users` record if administrator also needs staff portal.

## Receive administrator access

Deployment creates or updates initial PocketBase superuser from AWS SSM:

- `/wholesale/admin_email`
- `/wholesale/admin_password`

Obtain credentials through approved secret-sharing process. Do not send password through email or chat.

Admin panel should be network-restricted in `Caddyfile`. If `/_/` returns 403, connect from approved IP or VPN. If it is publicly reachable, fix Caddy allowlist immediately.

## First sign-in

1. Open `https://<domain>/_/`.
2. Sign in with PocketBase superuser credentials.
3. Confirm expected collections exist:
   - `users`
   - `customers`
   - `companies`
   - `orders`
   - `scheduledOrders`
4. Confirm application health and recent logs before changing schema or records.

Avoid editing collection schemas directly in production. Schema changes belong in new Go migration files and normal deployment process.

## Verify application settings

PocketBase settings are applied from environment at app bootstrap. Confirm:

- Application name and URL are correct.
- Sender name/address are correct.
- SMTP is enabled and valid.
- `users` and `customers` auth collections have OTP enabled and password auth disabled.

SMTP must work before staff or customers can sign in.

## Create a staff account

All records in PocketBase `users` collection are treated as staff. There is currently no separate staff/admin application role.

1. Open **Collections → users**.
2. Create record.
3. Enter staff email address exactly as they will use it.
4. If PocketBase form requires password, generate a strong random value. Staff password authentication is disabled; staff uses OTP.
5. Save record.
6. Give staff member application URL and [Staff Onboarding](staff-onboarding.md).

No staff welcome email is sent automatically. Their first OTP is sent when they choose **Staff** on login screen and request code.

### Remove staff access

Delete corresponding `users` record in PocketBase, then verify staff can no longer access protected routes. Review PocketBase auth/token settings when immediate session revocation needs confirmation.

## Customer account administration

Normal customer onboarding happens through staff portal, not PocketBase admin:

1. Customer must already exist in active Square environment.
2. Staff enters exact Square email and previews matching Square customer.
3. Staff confirms existing wholesale account or creates one. Square `company_name` is only a suggestion; never grant access from name matching alone.
4. Application creates local `customers` auth record referencing Square customer ID and wholesale account.
5. Customer receives welcome email and uses OTP login.

Square remains source of truth for customer contact and billing details. GCCR Wholesale remains source of truth for wholesale-account membership and shared access.

Account members can view account orders, invoices, and active schedules. Only schedule creator can cancel schedule. Orders and schedules snapshot account at creation, so later customer reassignment does not move historical records between accounts. First assignment of previously unassigned customer backfills only that customer's unassigned records.

Staff can change assignment from **Customers → Wholesale Account**. Review unassigned customers after deployment. Use PocketBase admin only for support or data repair. Before deleting or reassigning customer, check related orders and schedules.

## Operational checks

### Verify staff/customer email login

1. Confirm SMTP configuration.
2. Create test staff/customer in correct collection.
3. Request OTP from application login screen.
4. Check app logs and SMTP delivery logs.

### Verify Square configuration

Square token, location, wholesale category, customers, and webhook must belong to same selected environment:

```dotenv
SQUARE_SANDBOX=false
SQUARE_ACCESS_TOKEN=...
SQUARE_LOCATION_ID=...
SQUARE_WHOLESALE_CATEGORY_ID=...
```

### Verify webhook

Square Developer Dashboard must subscribe production webhook URL to `invoice.payment_made`. Successful webhook transitions matching invoiced order to `paid`.

## Reset administrator credentials

See [Deployment and Operations: Resetting PocketBase superuser credentials](deployment.md#resetting-pocketbase-superuser-credentials).

Changing SSM values alone does not change PocketBase. Run `superuser upsert` command against `/app/pb_data`.

## Security rules

- Never share superuser account among staff.
- Never expose `/_/` publicly.
- Never commit `.env`, tokens, SMTP credentials, or SSM values.
- Use staff portal for daily operations; reserve PocketBase admin for administration and support.
- Back up `pb_data` before schema or bulk-data changes.
- Add new migrations rather than editing applied migration files.
