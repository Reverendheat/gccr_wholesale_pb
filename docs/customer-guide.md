# Customer Guide

## Get access

GCCR Wholesale accounts are invitation-only.

Before invitation:

1. Your business/contact must exist as customer in Ground Control Roasters Square account.
2. GCCR staff links your login to correct wholesale account and invites email address stored in Square.
3. You receive welcome email with GCCR Wholesale link.

If welcome email does not arrive, contact GCCR staff. If account was created, you can still open portal directly and request one-time sign-in code.

## Sign in

1. Open GCCR Wholesale link.
2. Select **Customer**.
3. Enter same email address used for invitation.
4. Request sign-in code.
5. Retrieve code from email.
6. Enter code before it expires.

No password is needed. Each login uses one-time email code.

## Add to iPhone Home Screen

Ground Control Roasters shows an install prompt when opened on an iPhone or iPad browser. Select **Show me how**, then:

1. Tap **Share** in browser toolbar.
2. Select **Add to Home Screen**.
3. Enable **Open as Web App**, then tap **Add**.

Home Screen icon opens portal in standalone app window. Installed users do not see prompt again. Selecting **Not now** hides prompt for 30 days.

If code does not arrive:

- Confirm you selected **Customer**, not **Staff**.
- Confirm email matches invitation exactly.
- Check spam/junk folder.
- Ask GCCR staff to confirm customer account exists.

## Browse wholesale catalog

**Order** tab shows items currently assigned to configured wholesale category in Square. Only fixed-price variations are available.

For each item:

- Review name and description.
- Choose desired variation.
- Select **+ Add**.
- Adjust quantity in cart with `+` and `−`.

Catalog is shared among customers; it is not customized by company account. Contact GCCR staff if expected item is missing.

## Place one-time order

1. Add one or more variations to cart.
2. Adjust quantities.
3. Choose **Pickup** or **Delivery**. Pickup is selected by default.
4. For delivery, enter recipient, US delivery address, and optional delivery instructions.
5. Add order notes or special requests if needed.
6. Select **Place Order**.
7. Wait for success confirmation.

Order is created in GCCR Wholesale with catalog pricing locked at submission. New order starts as `pending`. Square order and invoice are created later when staff sends invoice. Delivery details remain in GCCR Wholesale for fulfillment. Address checks validate required fields and US ZIP format, not real-world deliverability.

Open **Account Orders** to view orders submitted by you and other members of your wholesale account:

- Order number, date, and submitter
- Item count and details
- Pickup or snapshotted delivery details
- Notes
- Current status
- Square invoice link when available

Select order row to expand item details.

## Create recurring order

1. Build cart as normal.
2. Enable **Set as a recurring order**.
3. Choose frequency:
   - Weekly
   - Every 2 weeks
   - Monthly
   - Quarterly
4. Select schedule button.

First order is placed immediately. Future orders are created automatically on shown next-order date using fulfillment details saved when schedule was created. Each generated order locks wholesale catalog pricing in effect on its creation date.

Open **Account Schedules** to see active schedules, creators, frequencies, and next dates for your wholesale account.

### Cancel recurring order

Only schedule creator can select **Cancel**. Other account members have read-only visibility.

Cancellation stops future scheduled orders. It does not cancel:

- First order placed when schedule was created
- Orders already created by schedule
- Existing Square invoices

Contact GCCR staff about existing order changes or cancellations.

## Order statuses

| Status | Meaning |
|---|---|
| `pending` | Received and awaiting staff confirmation |
| `confirmed` | Accepted for fulfillment |
| `delivered` | Marked delivered by staff |
| `invoiced` | Square invoice sent |
| `paid` | Square reported payment complete |
| `cancelled` | Order cancelled |
| `needs_review` | Staff needs to reconcile order or payment state |

## Pay invoice

When staff sends invoice:

1. Square emails invoice/payment request.
2. **Account Orders** shows **View invoice** link to all members of wholesale account.
3. Open link and follow Square payment instructions.
4. After payment, status updates to `paid` when Square webhook is processed.

If payment succeeded but portal does not update, contact GCCR staff with order number and Square receipt/invoice reference. Do not submit payment twice.

## Catalog and pricing notes

Catalog and pricing come from Square. Estimated cart total uses current fixed variation prices. Availability is not currently filtered by inventory or customer account. GCCR staff will contact you if item requires adjustment.

## Sign out

Select **Sign out** when finished, especially on shared device.
