/**
 * Formatting shared by every screen, so that a date or a duration reads the
 * same wherever it appears.
 */

const MINUTE = 60;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/**
 * Renders a timestamp relative to now for the recent past, and as a date
 * beyond a week.
 *
 * @param {string | null | undefined} value
 * @param {string} [never] what to say when there is no timestamp
 */
export function relative(value, never = "Never") {
  if (!value) return never;

  const date = new Date(value);
  const seconds = Math.round((Date.now() - date.getTime()) / 1000);

  if (seconds < MINUTE) return "Just now";
  if (seconds < HOUR) return `${Math.floor(seconds / MINUTE)} min ago`;
  if (seconds < DAY) {
    const hours = Math.floor(seconds / HOUR);
    return `${hours} hour${hours === 1 ? "" : "s"} ago`;
  }
  if (seconds < 2 * DAY) return "Yesterday";
  if (seconds < 7 * DAY) return `${Math.floor(seconds / DAY)} days ago`;

  return shortDate(value);
}

/** Renders a date as "Sep 12, 2026". */
export function shortDate(value) {
  if (!value) return "—";
  return new Date(value).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

/** Renders a timestamp as "Sep 12, 2026, 9:14 AM". */
export function dateTime(value) {
  if (!value) return "—";
  return new Date(value).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/**
 * The day a timestamp falls on, as a heading: "Today", "Yesterday",
 * "Mon, Sep 12", with the year only when it is not this one.
 */
export function dayLabel(value) {
  const date = new Date(value);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);
  if (date.toDateString() === today.toDateString()) return "Today";
  if (date.toDateString() === yesterday.toDateString()) return "Yesterday";

  return date.toLocaleDateString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
    year: date.getFullYear() === today.getFullYear() ? undefined : "numeric",
  });
}

/** Renders the time of day of a timestamp, as "6:20 PM". */
export function timeOfDay(value) {
  return new Date(value).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
}

/** Renders a number of seconds as "6d 4h", "3h 12m", "52m" or "40s". */
export function duration(seconds) {
  if (seconds < MINUTE) return `${seconds}s`;

  const days = Math.floor(seconds / DAY);
  const hours = Math.floor((seconds % DAY) / HOUR);
  const minutes = Math.floor((seconds % HOUR) / MINUTE);

  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

/**
 * Two letters for an avatar, from a name when there is one and from the
 * address otherwise.
 *
 * @param {string} [name]
 * @param {string} [email]
 */
export function initials(name, email) {
  const words = (name ?? "").trim().split(/\s+/).filter(Boolean);
  if (words.length >= 2) return (words[0][0] + words.at(-1)[0]).toUpperCase();
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();

  const local = (email ?? "").split("@")[0];
  const parts = local.split(/[._-]+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return local.slice(0, 2).toUpperCase() || "?";
}

const SYSTEMS = { macos: "macOS", windows: "Windows", linux: "Linux" };

/** The display name of a device's operating system, or "" when unknown. */
export function systemName(os) {
  return SYSTEMS[os] ?? "";
}

const UNITS = ["B", "KB", "MB", "GB", "TB"];

/** Renders a byte count as "1.2 GB", "350 KB" or "0 B". */
export function bytes(count) {
  let value = Number(count) || 0;
  let unit = 0;
  while (value >= 1000 && unit < UNITS.length - 1) {
    value /= 1000;
    unit++;
  }
  const digits = value < 10 && unit > 0 ? 1 : 0;
  return `${value.toFixed(digits)} ${UNITS[unit]}`;
}
