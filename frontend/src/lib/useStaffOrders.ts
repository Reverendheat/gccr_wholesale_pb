import { useEffect, useState } from "react";
import { fetchStaffOrders, type OrderListFilters, type OrderPage } from "./api";

export const defaultOrderFilters: OrderListFilters = {
  q: "", status: "", company: "", from: "", to: "", sort: "newest",
};

export function useStaffOrders(invoices = false) {
  const [filters, setFilters] = useState(defaultOrderFilters);
  const [page, setPage] = useState(1);
  const [data, setData] = useState<OrderPage>({ orders: [], page: 1, has_more: false });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let active = true;
    fetchStaffOrders(filters, page, invoices)
      .then((result) => { if (active) setData(result); })
      .catch((cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : "Could not load orders"); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [filters, page, invoices, revision]);

  function apply(next: OrderListFilters) {
    setLoading(true);
    setError(null);
    setPage(1);
    setFilters({ ...next });
  }

  function changePage(next: number) {
    setLoading(true);
    setError(null);
    setPage(next);
  }

  function refresh() {
    setLoading(true);
    setError(null);
    setRevision((value) => value + 1);
  }

  return { orders: data.orders, hasMore: data.has_more, page, loading, error, apply, changePage, refresh };
}
