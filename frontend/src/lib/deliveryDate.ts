export const DELIVERY_TIME_ZONE = "America/New_York";

const WEEKDAY_INDEX: Record<string, number> = {
  Sun: 0,
  Mon: 1,
  Tue: 2,
  Wed: 3,
  Thu: 4,
  Fri: 5,
  Sat: 6,
};

function zonedCalendarDate(now: Date, timeZone: string): { year: number; month: number; day: number; weekday: number } {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    weekday: "short",
  }).formatToParts(now);
  const value = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value ?? "";

  return {
    year: Number(value("year")),
    month: Number(value("month")),
    day: Number(value("day")),
    weekday: WEEKDAY_INDEX[value("weekday")] ?? 0,
  };
}

// Every delivery is Thursday. Monday night is the cutoff: Tuesday through
// Thursday roll to the following week's delivery, while Friday through Monday
// still target the next Thursday.
export function estimatedDeliveryDate(now: Date, timeZone = DELIVERY_TIME_ZONE): string {
  const local = zonedCalendarDate(now, timeZone);
  const daysUntilMonday = (8 - local.weekday) % 7;
  const delivery = new Date(Date.UTC(local.year, local.month - 1, local.day + daysUntilMonday + 3));
  return delivery.toISOString().slice(0, 10);
}

export function formatDeliveryDate(isoDate: string): string {
  return new Intl.DateTimeFormat("en-US", {
    timeZone: "UTC",
    weekday: "long",
    month: "long",
    day: "numeric",
  }).format(new Date(`${isoDate}T12:00:00Z`));
}

export function deliveryPreviewNow(
  search: string,
  allowOverride: boolean,
  fallback = new Date(),
): { now: Date; overridden: boolean } {
  if (!allowOverride) return { now: fallback, overridden: false };

  const raw = new URLSearchParams(search).get("deliveryNow");
  if (!raw) return { now: fallback, overridden: false };

  const override = new Date(raw);
  if (Number.isNaN(override.getTime())) return { now: fallback, overridden: false };
  return { now: override, overridden: true };
}
