Is there any reconciliation between the invoices and the order on square and this app? What happens if a change is made to the order on the square side? should we implement something like Go FSM?

Should customers be able to edit orders after they are submitted? what would that look like for the square entity that's created?

Fix existing frontend lint errors in `AuthContext.tsx`: initialize restored auth state without synchronous `setState` in an effect, and split `useAuth`/context exports from component exports for React Fast Refresh.
