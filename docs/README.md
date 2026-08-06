# GCCR Wholesale Documentation

Use guide matching your role:

- [Deployment and operations](deployment.md) — first deployment, updates, configuration, backups, and troubleshooting
- [Administrator onboarding](admin-onboarding.md) — PocketBase superuser access, initial checks, and staff account management
- [Staff onboarding](staff-onboarding.md) — staff sign-in, customer invitations, orders, and invoices
- [Customer guide](customer-guide.md) — customer sign-in, ordering, recurring orders, and invoices

## Roles

GCCR Wholesale has three distinct access levels:

| Role | PocketBase collection | Access |
|---|---|---|
| PocketBase administrator | `_superusers` | PocketBase administration UI at `/_/`; unrestricted database and configuration access |
| Staff | `users` | Staff portal: customers, orders, and invoices |
| Customer | `customers` | Customer portal: catalog, own orders, schedules, and invoices |

A PocketBase administrator is not automatically a staff user. Create a separate `users` record if an administrator also needs staff-portal access.
