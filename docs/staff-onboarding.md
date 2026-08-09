# Staff Onboarding and Use

## Before first sign-in

PocketBase administrator must create staff record in `users` collection using your email address. Staff accounts are not self-service and no welcome email is sent automatically.

You need:

- Application URL
- Access to staff email inbox
- Existing `users` record matching that email

## Sign in

1. Open application URL.
2. Select **Staff**.
3. Enter staff email.
4. Select **Send code**.
5. Retrieve one-time code from email.
6. Enter code before it expires.

Staff uses one-time codes, not passwords. If code does not arrive, confirm email spelling, check spam, then contact administrator to verify `users` record and SMTP delivery.

## Add to iPhone Home Screen

Ground Control Roasters shows an install prompt when opened on an iPhone or iPad browser. Select **Show me how**, then:

1. Tap **Share** in browser toolbar.
2. Select **Add to Home Screen**.
3. Enable **Open as Web App**, then tap **Add**.

Home Screen icon opens staff portal in standalone app window. Installed users do not see prompt again. Selecting **Not now** hides prompt for 30 days.

## Staff navigation

Staff portal contains:

- **Orders** — all customer orders and workflow actions
- **Invoices** — orders with Square invoices
- **Customers** — customer list and invitations

All staff records currently have same permissions.

## Invite a customer

Before invitation, customer must already exist in currently selected Square environment with exact email address.

1. In Square Dashboard, confirm customer exists in correct environment (production or sandbox).
2. Confirm Square customer has email, name, phone, and valid customer ID.
3. In GCCR Wholesale, open **Customers**.
4. Select **Invite Customer**.
5. Enter exact Square email.
6. Submit invitation.

Application then:

1. Searches Square by email.
2. Rejects invitation if no matching Square customer exists.
3. Rejects invitation if local PocketBase customer already uses email.
4. Creates local PocketBase customer account.
5. Stores Square customer ID as reference; it does not create a new Square customer.
6. Copies Square name and phone into local account.
7. Sends welcome email linking customer to application.

Customer signs in using one-time email code. No customer password is created for use.

### Invitation troubleshooting

| Message | Meaning/action |
|---|---|
| `This email isn't registered in Square` | Add customer in correct Square environment or use exact Square email |
| `A customer account already exists` | Do not invite again; customer can use OTP login |
| `Could not search Square for customer` | Check Square token/environment/API status |
| `Could not create customer account` | Administrator should inspect app logs and `customers` schema |

Welcome-email failure is logged after account creation and may not make invitation request fail. Customer can still open application and request OTP manually.

## Manage orders

Open **Orders**, then select row to view details:

- Customer information
- Local and Square order IDs
- Item variations and quantities
- Pickup or delivery details, including snapshotted address and instructions
- Notes
- Current status
- Invoice information

### Status workflow

| Status | Meaning | Available staff action |
|---|---|---|
| `pending` | New customer order | Confirm, cancel, or send invoice |
| `confirmed` | Accepted for fulfillment | Mark delivered, cancel, or send invoice |
| `delivered` | Fulfilled | Cancel or send invoice |
| `invoiced` | Square invoice sent | Wait for Square payment webhook |
| `paid` | Square reported invoice paid | No normal action |
| `cancelled` | Order cancelled | No normal action |
| `needs_review` | Square/local state requires manual review | Review records; UI allows cancellation |

Do not manually edit status in PocketBase. Use staff actions so workflow validation remains intact.

## Send an invoice

Invoice can be sent when order:

- Has Square order ID
- Does not already have Square invoice
- Is not `paid`, `cancelled`, or `needs_review`

From order detail:

1. Review customer, fulfillment details, items, notes, and status.
2. Select **Send invoice**.
3. Application creates Square invoice due 30 days from current date.
4. Square emails payment request to customer.
5. Invoice link appears in order detail and **Invoices** tab.
6. Square `invoice.payment_made` webhook changes local order status to `paid`.

Never send second invoice directly in Square without reconciling local order record.

## Invoices tab

Invoices tab lists orders with Square invoice ID. Use **View invoice** to open Square invoice URL. If payment happened but status is not `paid`, ask administrator to verify webhook delivery/signature and app logs.

## Customer support

### Customer did not receive welcome email

- Confirm invitation created local customer record.
- Ask customer to open application directly.
- Customer selects **Customer**, enters same email, and requests OTP.
- Administrator checks SMTP/app logs if OTP also fails.

### Customer cannot see expected items

Catalog includes only fixed-price variations in configured Square wholesale category. Ask administrator to verify:

- Correct Square environment
- Correct `SQUARE_WHOLESALE_CATEGORY_ID`
- Item assigned to category
- Variation has fixed price

### Customer wants to stop recurring order

Customer can cancel active schedule from **Scheduled Orders** tab. Cancellation stops future automatic orders; it does not cancel first order already created or previous orders.

## Sign out

Use **Sign out** in staff navigation when finished, especially on shared devices.
