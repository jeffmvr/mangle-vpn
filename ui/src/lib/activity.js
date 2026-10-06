import {
  Archive,
  CircleAlert,
  Download,
  KeyRound,
  LaptopMinimal,
  ListTree,
  LockOpen,
  LogIn,
  PlugZap,
  Power,
  Shield,
  ShieldOff,
  SlidersHorizontal,
  Trash2,
  Unplug,
  UserPen,
  UserPlus,
  Users,
  UserX,
} from "lucide-vue-next";

/**
 * How the audit log's entries are shown, shared by the Activity page and a
 * user's own Activity tab.
 */

// Administrators' changes share one mark, so they stand out from the
// traffic of sign-ins and connections around them.
const ADMIN = "text-accent-strong bg-accent-tint";

/** What each kind of event is called on screen, and how it is marked. */
export const KINDS = {
  "web.login": { label: "Signed in", icon: LogIn, tone: "text-ink-3 bg-muted" },
  "web.error": { label: "Sign-in failed", icon: CircleAlert, tone: "text-deny bg-deny-soft" },
  "vpn.connect": { label: "Connected", icon: PlugZap, tone: "text-up bg-up-soft" },
  "vpn.disconnect": { label: "Disconnected", icon: Unplug, tone: "text-ink-3 bg-muted" },
  "vpn.error": { label: "Connection refused", icon: CircleAlert, tone: "text-deny bg-deny-soft" },
  "device.import": { label: "Device imported", icon: Download, tone: "text-accent bg-accent-soft" },
  "device.create": { label: "Device added", icon: LaptopMinimal, tone: "text-accent bg-accent-soft" },
  "device.delete": { label: "Device removed", icon: Trash2, tone: "text-ink-3 bg-muted" },
  "device.retire": { label: "Unused device removed", icon: Trash2, tone: "text-warn bg-warn-soft" },
  "account.password": { label: "Password changed", icon: KeyRound, tone: "text-ink-3 bg-muted" },
  "account.create": { label: "Account created", icon: UserPlus, tone: "text-accent bg-accent-soft" },
  "admin.user.invite": { label: "Invited people", icon: UserPlus, tone: ADMIN },
  "admin.user.update": { label: "Changed a user", icon: UserPen, tone: ADMIN },
  "admin.user.delete": { label: "Deleted a user", icon: UserX, tone: ADMIN },
  "admin.user.password": { label: "Reset a password", icon: KeyRound, tone: ADMIN },
  "admin.user.mfa": { label: "Reset two-factor", icon: ShieldOff, tone: ADMIN },
  "admin.user.unlock": { label: "Unlocked an account", icon: LockOpen, tone: ADMIN },
  "admin.group.create": { label: "Created a group", icon: Users, tone: ADMIN },
  "admin.group.update": { label: "Changed a group", icon: Users, tone: ADMIN },
  "admin.group.delete": { label: "Deleted a group", icon: Users, tone: ADMIN },
  "admin.firewall.create": { label: "Added a firewall rule", icon: Shield, tone: ADMIN },
  "admin.firewall.update": { label: "Changed a firewall rule", icon: Shield, tone: ADMIN },
  "admin.firewall.delete": { label: "Removed a firewall rule", icon: Shield, tone: ADMIN },
  "admin.device.delete": { label: "Removed a device", icon: Trash2, tone: ADMIN },
  "admin.device.update": { label: "Changed a device", icon: LaptopMinimal, tone: ADMIN },
  "admin.client.disconnect": { label: "Disconnected a client", icon: Unplug, tone: ADMIN },
  "admin.settings": { label: "Changed settings", icon: SlidersHorizontal, tone: ADMIN },
  "admin.openvpn": { label: "Controlled OpenVPN", icon: Power, tone: ADMIN },
  "admin.backup": { label: "Backup", icon: Archive, tone: ADMIN },
};

/** The kinds of event the list can be narrowed to, as the API names them. */
export const FILTERS = [
  { value: "", label: "All activity" },
  { value: "web", label: "Sign-ins" },
  { value: "vpn", label: "Connections" },
  { value: "device", label: "Devices" },
  { value: "account", label: "Account changes" },
  { value: "admin", label: "Admin changes" },
];

/**
 * The part of an event's recorded text that the label does not already
 * say: the device and address for "Device MacBook connected from 1.2.3.4",
 * the reason for a refusal. The text is stored with the event, by this
 * release or the Django one, so anything unrecognised is shown whole.
 */
const SUMMARIES = [
  [/^Logged in to web application from (.+?)\.?$/, (ip) => `From ${ip}`],
  [/^Device (.+) connected from (.+?)\.?$/, (device, ip) => `${device}, from ${ip}`],
  [/^Device (.+) disconnected from (.+?) after (.+?)\.?$/, (device, ip, length) => `${device}, from ${ip}, after ${length}`],
  [/^Device (.+) added from OpenVPN Connect at (.+?)\.?$/, (device, ip) => `${device}, from ${ip}`],
  [/^Connection refused: (.)(.*)$/, (first, rest) => first.toUpperCase() + rest],
];

export function eventSummary(event) {
  const detail = event.detail ?? "";
  for (const [pattern, format] of SUMMARIES) {
    const match = detail.match(pattern);
    if (match) return format(...match.slice(1));
  }
  return detail;
}

/** An event's display details, falling back to its raw name. */
export function eventKind(event) {
  return KINDS[event.name] ?? { label: event.name, icon: ListTree, tone: "text-ink-3 bg-muted" };
}
