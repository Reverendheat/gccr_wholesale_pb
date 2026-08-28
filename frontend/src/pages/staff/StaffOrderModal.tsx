import { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  fetchFulfillmentOptions,
  fetchWholesaleCatalog,
  quoteFulfillment,
  submitStaffOrder,
  updateStaffOrder,
  type CatalogItem,
  type CatalogVariation,
  type GrindOption,
  type CustomerRecord,
  type Fulfillment,
  type FulfillmentOptions,
  type FulfillmentQuoteResult,
  type Order,
  type StaffOrderResult,
} from "../../lib/api";
import {
  groupCatalogByAudience,
  WHOLESALE_AUDIENCE_SECTIONS,
} from "../../lib/catalog";
import "./StaffOrderModal.css";

type CartItem = {
  itemName: string;
  variation: CatalogVariation;
  quantity: number;
  grindName?: string;
  grindModifierId?: string;
};

function cartItemKey(variationId: string, grindModifierId = ""): string {
  return `${variationId}:${grindModifierId}`;
}

function formatMoney(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function validDelivery(fulfillment: Fulfillment): boolean {
  return fulfillment.method === "delivery" && Boolean(
    fulfillment.recipient_name?.trim() &&
    fulfillment.recipient_phone?.trim() &&
    fulfillment.address_line_1?.trim() &&
    fulfillment.city?.trim() &&
    /^[A-Za-z]{2}$/.test(fulfillment.state?.trim() ?? "") &&
    /^\d{5}(-\d{4})?$/.test(fulfillment.postal_code?.trim() ?? ""),
  );
}

export default function StaffOrderModal({
  customer,
  onClose,
  onCreated,
  onUpdated,
  order,
}: {
  customer: CustomerRecord;
  onClose: () => void;
  onCreated?: (result: StaffOrderResult) => void;
  onUpdated?: (result: StaffOrderResult) => void;
  order?: Order;
}) {
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [options, setOptions] = useState<FulfillmentOptions | null>(null);
  const [cart, setCart] = useState<CartItem[]>([]);
  const [grindByVariation, setGrindByVariation] = useState<Record<string, string>>({});
  const [fulfillment, setFulfillment] = useState<Fulfillment>(order?.fulfillment ?? {
    method: "pickup",
    recipient_name: customer.name,
    recipient_phone: customer.phone,
    country: "US",
  });
  const [notes, setNotes] = useState(order?.notes ?? "");
  const [placementNote, setPlacementNote] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [quoteState, setQuoteState] = useState<{ key: string; result: FulfillmentQuoteResult } | null>(null);
  const [quoteError, setQuoteError] = useState<{ key: string; message: string } | null>(null);
  const [quotingKey, setQuotingKey] = useState<string | null>(null);
  const catalogByAudience = useMemo(
    () => groupCatalogByAudience(catalog),
    [catalog],
  );

  useEffect(() => {
    Promise.all([fetchWholesaleCatalog(customer.id), fetchFulfillmentOptions()])
      .then(([items, fulfillmentOptions]) => {
        setCatalog(items);
        setOptions(fulfillmentOptions);
        if (order) {
          const existingCart = order.lineItems.map((lineItem) => {
            for (const item of items) {
              const variation = item.item_data.variations.find((candidate) => candidate.id === lineItem.variation_id);
              if (variation) {
                return {
                  itemName: item.item_data.name,
                  variation,
                  quantity: lineItem.quantity,
                  grindName: lineItem.grind,
                  grindModifierId: lineItem.grind_modifier_id,
                };
              }
            }
            throw new Error(`Order item ${lineItem.name || lineItem.variation_id} is no longer available`);
          });
          setCart(existingCart);
        }
      })
      .catch((cause: unknown) => setError(cause instanceof Error ? cause.message : "Could not load order form"))
      .finally(() => setLoading(false));
  }, [customer.id, order]);

  const lineItems = useMemo(
    () => cart.map((item) => ({
      variation_id: item.variation.id,
      grind_modifier_id: item.grindModifierId,
      quantity: item.quantity,
    })),
    [cart],
  );
  const quoteKey = JSON.stringify({ lineItems, fulfillment });
  const quote = quoteState?.key === quoteKey ? quoteState.result : null;
  const activeQuoteError = quoteError?.key === quoteKey ? quoteError.message : null;
  const quoting = quotingKey === quoteKey;

  useEffect(() => {
    if (!validDelivery(fulfillment) || cart.length === 0) return;
    const requestKey = quoteKey;
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setQuotingKey(requestKey);
      setQuoteError(null);
      try {
        const result = await quoteFulfillment(lineItems, fulfillment, controller.signal, customer.id);
        setQuoteState({ key: requestKey, result });
      } catch (cause: unknown) {
        if (cause instanceof DOMException && cause.name === "AbortError") return;
        setQuoteError({ key: requestKey, message: cause instanceof Error ? cause.message : "Could not calculate delivery quote" });
      } finally {
        if (!controller.signal.aborted) {
          setQuotingKey((current) => current === requestKey ? null : current);
        }
      }
    }, 500);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [cart, customer.id, fulfillment, lineItems, quoteKey]);

  function addItem(item: CatalogItem, variation: CatalogVariation, grind?: GrindOption) {
    const grindModifierId = grind?.modifier_id ?? "";
    const key = cartItemKey(variation.id, grindModifierId);
    setCart((current) => {
      const existing = current.find(
        (entry) => cartItemKey(entry.variation.id, entry.grindModifierId) === key,
      );
      if (existing) {
        return current.map((entry) =>
          cartItemKey(entry.variation.id, entry.grindModifierId) === key
            ? { ...entry, quantity: entry.quantity + 1 }
            : entry
        );
      }
      return [...current, {
        itemName: item.item_data.name,
        variation,
        quantity: 1,
        grindName: grind?.name,
        grindModifierId: grind?.modifier_id,
      }];
    });
  }

  function updateQuantity(key: string, quantity: number) {
    setCart((current) => quantity <= 0
      ? current.filter((entry) => cartItemKey(entry.variation.id, entry.grindModifierId) !== key)
      : current.map((entry) =>
          cartItemKey(entry.variation.id, entry.grindModifierId) === key
            ? { ...entry, quantity }
            : entry
        ));
  }

  function updateFulfillment(field: keyof Fulfillment, value: string) {
    setFulfillment((current) => ({ ...current, [field]: value }));
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    if (cart.length === 0) return setError("Add at least one item");
    if (!placementNote.trim()) return setError("Reason or authorization note is required");
    if (fulfillment.method === "delivery" && !quote) {
      return setError(activeQuoteError ?? "Wait for valid delivery quote");
    }
    const action = order ? "Update" : "Create";
    if (!window.confirm(`${action} order for ${customer.name || customer.email}? Customer will see this order immediately.`)) return;

    setSubmitting(true);
    try {
      const result = order
        ? await updateStaffOrder(order.id, lineItems, notes, fulfillment, placementNote.trim())
        : await submitStaffOrder(customer.id, lineItems, notes, fulfillment, placementNote.trim());
      if (order) onUpdated?.(result);
      else onCreated?.(result);
      onClose();
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : "Could not create order");
    } finally {
      setSubmitting(false);
    }
  }

  const subtotal = cart.reduce((sum, item) =>
    sum + (item.variation.item_variation_data.price_money?.amount ?? 0) * item.quantity, 0);
  const deliveryReady = fulfillment.method === "pickup" || Boolean(quote);

  return (
    <div className="staff-order-backdrop" onClick={onClose}>
      <div className="staff-order-modal" onClick={(event) => event.stopPropagation()}>
        <div className="staff-order-header">
          <div>
            <h3>{order ? "Edit order" : "Create order"}</h3>
            <p>{customer.name || customer.email} · {customer.expand?.company?.name || "Unassigned account"}</p>
          </div>
          <button type="button" className="drawer-close" onClick={onClose} aria-label="Close">✕</button>
        </div>

        {loading ? <p className="muted">Loading catalog…</p> : (
          <form onSubmit={handleSubmit} className="staff-order-form">
            <section>
              {WHOLESALE_AUDIENCE_SECTIONS.map(({ audience, label }) => (
                <div key={audience}>
                  <h4>{label}</h4>
                  <div className="staff-order-catalog">
                    {catalogByAudience[audience].flatMap((item) =>
                      item.item_data.variations.map((variation) => {
                        const grindOptions = item.grind_options ?? [];
                        const selectedModifierId = grindByVariation[variation.id] ??
                          grindOptions[0]?.modifier_id ?? "";
                        const selectedGrind = grindOptions.find(
                          (option) => (option.modifier_id ?? "") === selectedModifierId,
                        );
                        return (
                          <div className="staff-order-catalog-item" key={variation.id}>
                            <span>{item.item_data.name} — {variation.item_variation_data.name}</span>
                            <strong>{formatMoney(variation.item_variation_data.price_money?.amount ?? 0)}</strong>
                            {grindOptions.length > 0 && (
                              <label>
                                Grind
                                <select
                                  value={selectedModifierId}
                                  onChange={(event) => setGrindByVariation((current) => ({
                                    ...current,
                                    [variation.id]: event.target.value,
                                  }))}
                                >
                                  {grindOptions.map((option) => (
                                    <option key={option.modifier_id ?? "whole-bean"} value={option.modifier_id ?? ""}>
                                      {option.name}
                                    </option>
                                  ))}
                                </select>
                              </label>
                            )}
                            <button type="button" onClick={() => addItem(item, variation, selectedGrind)}>
                              Add
                            </button>
                          </div>
                        );
                      })
                    )}
                  </div>
                </div>
              ))}
            </section>

            <section>
              <h4>Order</h4>
              {cart.length === 0 ? <p className="muted">No items added.</p> : cart.map((item) => {
                const key = cartItemKey(item.variation.id, item.grindModifierId);
                return (
                  <div className="staff-order-cart-row" key={key}>
                    <span>
                      {item.itemName} — {item.variation.item_variation_data.name}
                      {item.grindName && ` · ${item.grindName}`}
                    </span>
                    <div>
                      <button type="button" onClick={() => updateQuantity(key, item.quantity - 1)}>−</button>
                      <span>{item.quantity}</span>
                      <button type="button" onClick={() => updateQuantity(key, item.quantity + 1)}>+</button>
                    </div>
                  </div>
                );
              })}
              <p className="staff-order-total">Merchandise subtotal: {formatMoney(subtotal)}</p>
              {quote && (
                <p className="staff-order-total">
                  Delivery: {formatMoney(quote.fulfillment.fee_cents ?? 0)} · Total: {formatMoney(quote.total_cents)}
                </p>
              )}
            </section>

            <section>
              <h4>Fulfillment</h4>
              <div className="staff-order-methods">
                <label><input type="radio" name="staff-fulfillment" checked={fulfillment.method === "pickup"} onChange={() => updateFulfillment("method", "pickup")} /> Pickup</label>
                <label><input type="radio" name="staff-fulfillment" checked={fulfillment.method === "delivery"} onChange={() => updateFulfillment("method", "delivery")} /> Delivery{options && ` (within ${options.max_miles} driving miles)`}</label>
              </div>
              {fulfillment.method === "delivery" && (
                <div className="staff-order-fields two-column">
                  <label>Recipient name *<input value={fulfillment.recipient_name ?? ""} onChange={(e) => updateFulfillment("recipient_name", e.target.value)} required /></label>
                  <label>Recipient phone *<input value={fulfillment.recipient_phone ?? ""} onChange={(e) => updateFulfillment("recipient_phone", e.target.value)} required /></label>
                  <label className="wide">Address line 1 *<input value={fulfillment.address_line_1 ?? ""} onChange={(e) => updateFulfillment("address_line_1", e.target.value)} required /></label>
                  <label className="wide">Address line 2<input value={fulfillment.address_line_2 ?? ""} onChange={(e) => updateFulfillment("address_line_2", e.target.value)} /></label>
                  <label>City *<input value={fulfillment.city ?? ""} onChange={(e) => updateFulfillment("city", e.target.value)} required /></label>
                  <label>State *<input maxLength={2} value={fulfillment.state ?? ""} onChange={(e) => updateFulfillment("state", e.target.value.toUpperCase())} required /></label>
                  <label>ZIP code *<input value={fulfillment.postal_code ?? ""} onChange={(e) => updateFulfillment("postal_code", e.target.value)} required /></label>
                  <label className="wide">Delivery instructions<textarea value={fulfillment.instructions ?? ""} onChange={(e) => updateFulfillment("instructions", e.target.value)} /></label>
                  {quoting && <p className="wide muted">Calculating delivery quote…</p>}
                  {activeQuoteError && <p className="wide staff-error">{activeQuoteError}</p>}
                  {quote && <p className="wide staff-success">{quote.fulfillment.distance_miles?.toFixed(1)} driving miles · {quote.fulfillment.fee_cents ? `${formatMoney(quote.fulfillment.fee_cents)} fee` : "Free delivery"}</p>}
                </div>
              )}
            </section>

            <section className="staff-order-fields">
              <label>
                {order ? "Edit reason" : "Reason / authorization"} *
                <input value={placementNote} onChange={(e) => setPlacementNote(e.target.value)} maxLength={500} placeholder={order ? "Customer requested an additional item or other correction" : "Phone order, email request, or other authorization"} required />
              </label>
              <label>
                Customer-visible order notes
                <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={3} />
              </label>
            </section>

            {error && <p className="staff-error">{error}</p>}
            <p className="staff-order-warning">Customer will see this {order ? "update" : "order"} immediately and receive a confirmation email.</p>
            <div className="modal-actions">
              <button type="button" className="btn-secondary" onClick={onClose}>Cancel</button>
              <button type="submit" disabled={submitting || !deliveryReady || cart.length === 0}>
                {submitting ? (order ? "Updating…" : "Creating…") : (order ? "Update order" : "Create order")}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
