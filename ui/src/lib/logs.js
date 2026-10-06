/**
 * The server's logs, in the order the navigation lists them. `slug` is the
 * address, `id` the API's name for the log.
 */
export const LOG_SOURCES = [
  {
    slug: "application",
    id: "app",
    label: "Application",
    title: "Application log",
    description: "What the application has been doing: requests, sign-ins, background jobs and errors.",
  },
  {
    slug: "openvpn",
    id: "openvpn",
    label: "OpenVPN",
    title: "OpenVPN log",
    description: "The VPN server's own output: connections, handshakes and errors.",
  },
];

/**
 * Splits a logfmt line, as the server writes them, into its key=value
 * pairs. Values may be quoted, with backslash escapes inside.
 */
function logfmt(line) {
  const pairs = [];
  const pattern = /([^\s=]+)=("(?:[^"\\]|\\.)*"|\S*)/g;
  for (const match of line.matchAll(pattern)) {
    let value = match[2];
    if (value.startsWith('"')) {
      try {
        value = JSON.parse(value);
      } catch {
        value = value.slice(1, -1);
      }
    }
    pairs.push([match[1], value]);
  }
  return pairs;
}

const OPENVPN_TIME = /^(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2})\S*\s+(?:us=\d+\s+)?(.*)$/;

/**
 * Reads one log line into its time, level, message and remaining fields.
 * The server's own logs are logfmt; OpenVPN's start with a timestamp, and
 * its errors are found by their wording. Anything else is kept as it is.
 *
 * @returns {{ time: Date | null, level: string, message: string, fields: [string, string][], raw: string }}
 */
export function parseLogLine(raw) {
  if (raw.startsWith("time=")) {
    const pairs = logfmt(raw);
    const entry = { time: null, level: "INFO", message: "", fields: [], raw };
    for (const [key, value] of pairs) {
      if (key === "time") entry.time = new Date(value);
      else if (key === "level") entry.level = value.toUpperCase();
      else if (key === "msg") entry.message = value;
      else entry.fields.push([key, value]);
    }
    return entry;
  }

  const match = raw.match(OPENVPN_TIME);
  const message = match ? match[2] : raw;
  let level = "";
  if (/\b(ERROR|FATAL|failed|error)\b/.test(message)) level = "ERROR";
  else if (/\bWARNING\b/.test(message)) level = "WARN";
  return {
    time: match ? new Date(match[1].replace(" ", "T")) : null,
    level,
    message,
    fields: [],
    raw,
  };
}
