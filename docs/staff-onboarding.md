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

Staff uses one-time codes, not passwords. Email capitalization does not matter. If code does not arrive, confirm email spelling, check spam, then contact administrator to verify `users` record and SMTP delivery.

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

On phones, navigation moves to top of screen and table rows become labeled cards. Tap an order card to open full-screen order details.

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

## Place an order for a customer

Use this workflow only after customer authorizes a one-time order by phone, email, or another channel:

1. Open **Customers**.
2. Find linked customer and select **Create order**.
3. Add catalog items and quantities.
4. Choose pickup or delivery. Delivery uses same configured driving-radius and fee rules as customer checkout.
5. Enter required **Reason / authorization** note.
6. Review customer, merchandise subtotal, delivery fee, and total.
7. Confirm creation.

Order starts `pending`, appears in customer account immediately, and records staff identity plus authorization note for audit. Customer receives confirmation email; UI warns staff if email delivery fails. Staff-entered orders are intentionally one-time only. Never impersonate customer or handle customer payment credentials.

### Last-minute order corrections

Staff can select **Edit order** for customer-created or staff-created orders when all are true:

- Status is `pending` or `confirmed`
- No Square order exists
- No Square invoice exists

Use this for authorized corrections such as adding an item before delivery leaves. Staff must enter edit reason. Application re-locks catalog prices, recalculates delivery, preserves order status, appends staff/timestamp/reason audit entry, and emails customer. Once sent to Square, cancel/recreate or use future add-on workflow instead; never change local totals behind published invoice.

## Manage orders

Open **Orders**, then select row to view details:

- Customer information
- Local order ID; Square order ID after invoicing
- Item variations, locked submission prices, and quantities
- Pickup or delivery details, including snapshotted address, driving distance, fee, and instructions
- Placement actor and staff authorization note when applicable
- Notes
- Current status
- Invoice information

### Status workflow

| Status | Meaning | Available staff action |
|---|---|---|
| `pending` | New customer order | Confirm, cancel, or send invoice |
| `confirmed` | Accepted for fulfillment | Mark delivered, cancel, or send invoice |
| `delivered` | Fulfilled | Cancel or send invoice |
| `invoiced` | Square invoice published; check email delivery separately | Retry unsent emails if needed; wait for Square payment webhook |
| `paid` | Square reported invoice paid | No normal action |
| `cancelled` | Order cancelled | No normal action |
| `needs_review` | Square/local state requires manual review | Review records; UI allows cancellation |

Do not manually edit records in PocketBase. Direct collection updates are locked; use staff actions and **Edit order** so workflow, price locking, delivery calculations, and audit history remain intact.

## Send an invoice

A new invoice can be created for a `pending`, `confirmed`, or `delivered` order that does not already have an invoice attempt.

To configure recipients, open **Customers → Manage billing** for the wholesale account. Add one or more billing emails, or remove all addresses to use the order buyer's portal account email. A configured list replaces the buyer's email; the buyer is not automatically copied. Billing settings apply to the company attached to the order, even if the buyer later moves to another account.

From order detail:

1. Review customer, fulfillment details, locked prices, items, notes, and status.
2. Review the displayed invoice recipients, then select **Send invoice**.
3. Application creates the Square order from the locked snapshot, adds any `Local delivery` fee line, and creates an invoice due in 30 days.
4. The portal emails the same Square payment link individually to the selected recipients using its configured mail service. Square does not send an additional invoice email.
5. Invoice link and email delivery progress appear in order detail and the **Invoices** tab.
6. Square webhook updates local status after payment, cancellation, or refund.

If creation or publication fails, **Resume invoice** continues the saved attempt. If an email fails, **Retry unsent emails** sends only to the remaining recipients, using a refreshed link to the same invoice. Already-sent recipients are not intentionally emailed again. “Sent” means the mail service accepted the message, not that it reached the inbox.

Recipients are saved when the invoice attempt starts; later company billing changes do not redirect that invoice. Existing invoices created before portal delivery remain managed by Square and cannot be retried through this email flow.

New invoices use Square's manual-sharing mode: automatic invoice receipts and update/cancellation emails are disabled. The portal does not configure reminders. Do not add Square reminders to these invoices: Square sends them to the original Square customer, not the portal billing list.

Do not edit order line items directly in Square. Cancel and recreate invoice through normal workflow when correction is required. Never send second invoice directly in Square without reconciling local order record.

## Invoices tab

The **Invoices** tab lists published invoices and saved attempts, with recipient and email delivery status. Open the order to resume an attempt or retry unsent emails. Use **View invoice** to open the Square payment page. If payment happened but status is not `paid`, ask an administrator to verify webhook delivery/signature and app logs.

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
