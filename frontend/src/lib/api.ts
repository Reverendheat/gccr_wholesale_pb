import pb from "./pb";

const BASE = "/api/wholesale";

// ---------------------------------------------------------------------------
// Staff helpers — use the PocketBase JS SDK directly so we get relation
// expansion and the standard collection API without a custom backend route.
// ---------------------------------------------------------------------------

export interface CompanyRecord {
  id: string;
  name: string;
}

export interface CustomerRecord {
  id: string;
  name: string;
  email: string;
  phone: string;
  squareCustomerId: string;
  company: string;
  created: string;
  expand?: {
    company?: CompanyRecord;
  };
}

export async function fetchCustomers(): Promise<CustomerRecord[]> {
  return pb.collection("customers").getFullList<CustomerRecord>({
    sort: "name",
    expand: "company",
    requestKey: null, // disable auto-cancellation so StrictMode double-mount doesn't abort the request
  });
}

export async function fetchCompanies(): Promise<CompanyRecord[]> {
  return pb.collection("companies").getFullList<CompanyRecord>({
    sort: "name",
    requestKey: null,
  });
}

export type OrderEvent =
  | "staff_confirm"
  | "staff_mark_delivered"
  | "staff_cancel";

export async function sendOrderEvent(
  id: string,
  event: OrderEvent,
): Promise<Order> {
  const res = await fetch(`${BASE}/orders/${id}/events`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify({ event }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `Order update failed: ${res.status}`);
  }
  const data = await res.json();
  return data.order;
}

export interface SendInvoiceResult {
  order: Order;
  invoice_url: string;
}

export async function sendInvoice(orderId: string): Promise<SendInvoiceResult> {
  const res = await fetch(`${BASE}/invoices`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify({ order_id: orderId }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `Send invoice failed: ${res.status}`);
  }
  return res.json();
}

function authHeaders(): Record<string, string> {
  return pb.authStore.token
    ? { Authorization: pb.authStore.token }
    : {};
}

export interface CatalogVariation {
  id: string;
  type: string;
  item_variation_data: {
    item_id: string;
    name: string;
    pricing_type: "FIXED_PRICING" | "VARIABLE_PRICING";
    price_money?: { amount: number; currency: string };
  };
}

export interface CatalogItem {
  id: string;
  type: string;
  item_data: {
    name: string;
    description: string;
    variations: CatalogVariation[];
  };
}

export interface LineItemInput {
  variation_id: string;
  quantity: number;
  note?: string;
  name?: string;
  unit_price_cents?: number;
  currency?: string;
}

export interface SubmittedBy {
  id: string;
  name: string;
}

export type FulfillmentMethod = "pickup" | "delivery";

export interface Fulfillment {
  method: FulfillmentMethod;
  recipient_name?: string;
  recipient_phone?: string;
  address_line_1?: string;
  address_line_2?: string;
  city?: string;
  state?: string;
  postal_code?: string;
  country?: "US";
  instructions?: string;
}

export interface Order {
  id: string;
  customer: string;
  company: string;
  submittedBy?: SubmittedBy;
  status: string;
  notes: string;
  fulfillment: Fulfillment;
  lineItems: LineItemInput[];
  squareOrderId: string;
  squareInvoiceId: string;
  squareInvoiceUrl: string;
  created: string;
  expand?: {
    customer?: {
      id: string;
      name: string;
      email: string;
      phone: string;
    };
  };
}

export async function fetchStaffOrders(): Promise<Order[]> {
  const res = await fetch(`${BASE}/orders`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Orders fetch failed: ${res.status}`);
  const data = await res.json();
  return data.orders ?? [];
}

export type ScheduleFrequency = "weekly" | "biweekly" | "monthly" | "quarterly";

export interface ScheduledOrder {
  id: string;
  customer: string;
  company: string;
  submittedBy?: SubmittedBy;
  frequency: ScheduleFrequency;
  lineItems: LineItemInput[];
  notes: string;
  fulfillment: Fulfillment;
  next_run_at: string;
  active: boolean;
  created: string;
}

export async function fetchWholesaleCatalog(): Promise<CatalogItem[]> {
  const res = await fetch(`${BASE}/catalog`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Catalog fetch failed: ${res.status}`);
  const data = await res.json();
  return data.items ?? [];
}

export async function submitOrder(
  lineItems: LineItemInput[],
  notes: string,
  fulfillment: Fulfillment,
): Promise<Order> {
  const res = await fetch(`${BASE}/orders`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify({ lineItems: lineItems, notes, fulfillment }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `Order failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchOrders(): Promise<Order[]> {
  const res = await fetch(`${BASE}/orders`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Orders fetch failed: ${res.status}`);
  const data = await res.json();
  return data.orders ?? [];
}

export async function submitScheduledOrder(
  lineItems: LineItemInput[],
  notes: string,
  frequency: ScheduleFrequency,
  fulfillment: Fulfillment,
): Promise<{ order: Order; scheduledOrder: ScheduledOrder }> {
  const res = await fetch(`${BASE}/scheduled-orders`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify({ lineItems: lineItems, notes, frequency, fulfillment }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `Scheduled order failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchScheduledOrders(): Promise<ScheduledOrder[]> {
  const res = await fetch(`${BASE}/scheduled-orders`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Scheduled orders fetch failed: ${res.status}`);
  const data = await res.json();
  return data.scheduledOrders ?? [];
}

export async function cancelScheduledOrder(id: string): Promise<void> {
  const res = await fetch(`${BASE}/scheduled-orders/${id}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Cancel failed: ${res.status}`);
}

export interface SquareCustomerPreview {
  id: string;
  name: string;
  email: string;
  phone: string;
  company_name: string;
}

export interface PreviewCustomerResult {
  customer: SquareCustomerPreview;
  suggested_accounts: CompanyRecord[];
}

export async function previewCustomer(email: string): Promise<PreviewCustomerResult> {
  const res = await fetch(`${BASE}/customers/preview`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify({ email }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `Square lookup failed: ${res.status}`);
  }
  return res.json();
}

export interface AccountSelection {
  company_id?: string;
  new_company_name?: string;
}

export interface InviteResult {
  id: string;
  email: string;
  name: string;
  phone: string;
  squareCustomerId: string;
  company: string;
  expand: { company: CompanyRecord };
}

export async function inviteCustomer(
  email: string,
  account: AccountSelection,
): Promise<InviteResult> {
  const res = await fetch(`${BASE}/invite`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify({ email, ...account }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `Invite failed: ${res.status}`);
  }
  return res.json();
}

export async function assignCustomerAccount(
  customerId: string,
  account: AccountSelection,
): Promise<CustomerRecord> {
  const res = await fetch(`${BASE}/customers/${customerId}/account`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify(account),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `Account assignment failed: ${res.status}`);
  }
  const data = await res.json();
  return data.customer;
}
