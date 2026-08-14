# TODOs

Last reviewed: 2026-08-13

- [ ] Plan Square order/invoice reconciliation. Decide how app handles Square-side order changes and whether lifecycle needs a Go FSM.
- [ ] Decide whether customers can edit submitted orders and how edits update corresponding Square entities.
- [ ] Fix existing frontend lint errors in `AuthContext.tsx`: initialize restored auth state without synchronous `setState` in an effect, and split `useAuth`/context exports from component exports for React Fast Refresh.
- [ ] Migrate wholesale catalog to Hilltop:
  - Confirm Hilltop Square location ID.
  - Create new wholesale category and replacement items/variations.
  - Restrict each replacement item and variation to Hilltop.
  - Update `SQUARE_WHOLESALE_CATEGORY_ID` and `SQUARE_LOCATION_ID` together.
  - Test catalog display, ordering, fulfillment, and Square records before cutover.
  - Archive old `Wholesale Coffee` items after successful cutover; do not delete them, preserving order/reporting history.
- [ ] Design lightweight sales CRM and sample tracking:
  - Use one company/account record through its full lifecycle: prospect, active customer, inactive/former customer, or lost prospect.
  - Keep `customers` as individual authenticated portal users linked to that company/account; do not create duplicate prospect/customer company records.
  - Track assigned sales rep, pipeline state, access/account state, next follow-up, last activity, and lost/inactive reason.
  - Add activity timeline for sample drop-offs, calls, emails, visits, notes, products/quantities, outcomes, and follow-up dates.
  - Define offboarding when a customer leaves: mark company inactive/former, revoke portal access, stop schedules, retain orders/invoices/activity history, and allow later reactivation.
  - Decide whether sample drop-offs should create Hilltop inventory adjustments; do not represent samples as customer orders by default.
- [ ] Configure and validate payment-method-based invoice pricing:
  - Keep both card and ACH enabled on Square invoices (already enabled in `internal/square/invoices.go`).
  - Confirmed `Create card surcharge` is available in Square Dashboard; configure it for Hilltop at **Settings → Account & Settings → Payments → Service charges**, capped at 3%. Do not create a generic service charge or unconditional order line item.
  - Verify API-created Square invoices apply surcharge only when paid by credit card, with 0% surcharge for ACH and debit cards.
  - Test full payment, refunds, cancellations, reconciliation, receipts, taxes, and accounting exports before production rollout.
  - Review applicable card-network, disclosure, signage, and state-law requirements before enabling.
