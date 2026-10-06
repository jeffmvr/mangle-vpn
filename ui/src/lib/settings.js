/**
 * The sections of the settings page, in the order the navigation lists
 * them. `slug` is the address, `id` the API's name for the section.
 */
export const SETTINGS_SECTIONS = [
  {
    slug: "general",
    id: "app",
    label: "General",
    description: "Your organization, the web server's address and certificates, and how long records are kept.",
  },
  {
    slug: "sso",
    id: "auth",
    label: "Single sign-on",
    description: "Let people sign in with an account they already have, through Google or your identity provider.",
  },
  {
    slug: "email",
    id: "mail",
    label: "Email",
    description: "The mail server used for invitations and password resets.",
  },
  {
    slug: "openvpn",
    id: "vpn",
    label: "OpenVPN",
    description: "How devices reach the VPN and what it gives them once connected.",
  },
  {
    slug: "security",
    id: "security",
    label: "Security",
    description: "How sign-in is protected, how long sessions last, and where administration is open from.",
  },
  {
    slug: "backups",
    id: "backups",
    label: "Backups",
    description: "Nightly backups of the database, keys and certificates, and a copy kept off this machine.",
  },
  {
    slug: "alerts",
    id: "alerts",
    label: "Alerts",
    description: "Who hears about it when the server needs attention, and what for.",
  },
];
