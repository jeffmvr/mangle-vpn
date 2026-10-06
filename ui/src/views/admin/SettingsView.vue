<script setup>
import { computed, onUnmounted, ref, watch } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute } from "vue-router";

import AppButton from "@/components/ui/AppButton.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import SettingRow from "@/components/ui/SettingRow.vue";
import StatusDot from "@/components/ui/StatusDot.vue";
import ToggleSwitch from "@/components/ui/ToggleSwitch.vue";
import { SETTINGS_SECTIONS } from "@/lib/settings";
import api, { errorMessage, fieldErrors } from "@/lib/api";
import { bytes, dateTime, relative, shortDate } from "@/lib/format";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

/**
 * The application's settings, one section per page. Each section is read
 * and saved on its own, as the server stores them.
 */
const route = useRoute();
const session = useSession();
const toast = useToast();

// The section shown comes from the address, chosen in the sidebar. `tab`
// is the API's name for it.
const section = computed(() => SETTINGS_SECTIONS.find((item) => item.slug === route.params.section) ?? SETTINGS_SECTIONS[0]);
const tab = computed(() => section.value.id);

const settings = ref(null);
const interfaces = ref({});
const errors = ref({});
const saving = ref(false);

// The section as last loaded or saved, to tell whether anything has been
// edited since.
const saved = ref("");
const dirty = computed(() => settings.value !== null && JSON.stringify(settings.value) !== saved.value);

/** Puts the form back as it was last loaded or saved. */
function discard() {
  settings.value = JSON.parse(saved.value);
  errors.value = {};
}

const testAddress = ref("");
const testError = ref("");
const testing = ref(false);

/**
 * Switches stored as "True" or "False". The setter writes the same strings
 * back, or the getter would read its own write as off and the switch would
 * spring back.
 */
const redirectGateway = computed({
  get: () => settings.value?.vpn_redirect_gateway === "True",
  set: (value) => {
    settings.value.vpn_redirect_gateway = value ? "True" : "False";
  },
});

const letsEncrypt = computed({
  get: () => settings.value?.app_letsencrypt === "True",
  set: (value) => {
    settings.value.app_letsencrypt = value ? "True" : "False";
  },
});

const certificates = ref([]);

/** The certificates the server depends on, with their expiry dates. */
async function loadCertificates() {
  try {
    const { data } = await api.get("/admin/certificates");
    certificates.value = data;
  } catch {
    certificates.value = [];
  }
}

const renewing = ref(false);

/** Issues the OpenVPN server a new certificate from the same authority. */
async function renewVPNCertificate() {
  try {
    const { data } = await api.post("/admin/certificates/openvpn/renew");
    certificates.value = data;
    await session.refreshVpn();
    toast.success("Certificate renewed. Restart OpenVPN to start using it.");
  } catch (error) {
    toast.error(errorMessage(error, "The certificate could not be renewed."));
    throw error;
  }
}
const requirePassword = computed({
  get: () => settings.value?.vpn_require_password === "True",
  set: (value) => {
    settings.value.vpn_require_password = value ? "True" : "False";
  },
});

/** A "True"/"False" setting as a switch's boolean. */
function flag(name) {
  return computed({
    get: () => settings.value?.[name] === "True",
    set: (value) => {
      settings.value[name] = value ? "True" : "False";
    },
  });
}

const alertVPNDown = flag("alert_vpn_down");
const alertCertificates = flag("alert_certificates");
const alertLockouts = flag("alert_lockouts");
const alertNewDevice = flag("alert_new_device");
const alertAdminSignIn = flag("alert_admin_signin");
const alertNewAddress = flag("alert_new_address");
const alertDailyDigest = flag("alert_daily_digest");
const ssoOnly = flag("oauth2_only");
const portShare = flag("vpn_port_share");
const singleSession = flag("vpn_single_session");
const passwordComplexity = flag("auth_password_complexity");

// The groups an account made on first single sign-on can go into.
const groups = ref([]);

async function loadGroups() {
  const { data } = await api.get("/admin/groups/all");
  groups.value = data;
}

const LOGO_LIMIT = 256 * 1024;
const logoError = ref("");

/** Reads a chosen image into the logo setting, as a data URL. */
function chooseLogo(event) {
  const file = event.target.files?.[0];
  event.target.value = "";
  logoError.value = "";
  if (!file) return;
  if (file.size > LOGO_LIMIT) {
    logoError.value = "That image is too large. Keep it under 256 KB.";
    return;
  }
  const reader = new FileReader();
  reader.onload = () => (settings.value.app_logo = reader.result);
  reader.readAsDataURL(file);
}

// The backups kept on this machine and how the latest one went.
const backups = ref(null);
const backingUp = ref(false);

async function loadBackups() {
  const { data } = await api.get("/admin/backups");
  backups.value = data;
}

async function backUpNow() {
  backingUp.value = true;
  try {
    const { data } = await api.post("/admin/backups");
    toast.success(data.offsite ? "Backup taken and copied to the bucket." : "Backup taken.");
  } catch (error) {
    toast.error(errorMessage(error, "The backup failed."));
  } finally {
    backingUp.value = false;
    await loadBackups();
  }
}

const alertTesting = ref(false);
const alertTestError = ref("");

async function sendTestAlert() {
  alertTesting.value = true;
  alertTestError.value = "";
  try {
    await api.post("/admin/settings/alerts/test");
    toast.success("Test alert sent.");
  } catch (error) {
    alertTestError.value = fieldErrors(error).alert_emails ?? errorMessage(error);
  } finally {
    alertTesting.value = false;
  }
}

const deviceToDevice = computed({
  get: () => settings.value?.vpn_device_to_device === "True",
  set: (value) => {
    settings.value.vpn_device_to_device = value ? "True" : "False";
  },
});

async function load(section) {
  settings.value = null;
  errors.value = {};
  const { data } = await api.get(`/admin/settings/${section}`);
  interfaces.value = data.interfaces ?? {};
  delete data.interfaces;
  settings.value = data;
  saved.value = JSON.stringify(data);
}

async function save() {
  saving.value = true;
  errors.value = {};
  try {
    await api.put(`/admin/settings/${tab.value}`, settings.value);
    toast.success(
      tab.value === "vpn" ? "Saved. Restart OpenVPN to apply the changes." : "Settings saved.",
    );
    if (tab.value === "app") await session.loadInfo();
    if (tab.value === "vpn") await session.refreshVpn();
    await load(tab.value);
  } catch (error) {
    errors.value = fieldErrors(error);
    if (!Object.keys(errors.value).some((key) => key in settings.value)) toast.error(errorMessage(error));
  } finally {
    saving.value = false;
  }
}

async function sendTest() {
  testing.value = true;
  testError.value = "";
  try {
    await api.post("/admin/settings/mail/test", { email: testAddress.value });
    toast.success(`Test email sent to ${testAddress.value}.`);
  } catch (error) {
    testError.value = fieldErrors(error).email ?? errorMessage(error);
  } finally {
    testing.value = false;
  }
}

// Leaving with unsaved edits, by the sidebar or any other link, asks first.
// The guard waits on the dialog: confirming leaves, closing it stays.
const leaving = ref(false);
let decide = null;

function confirmLeave() {
  if (!dirty.value) return true;
  leaving.value = true;
  return new Promise((resolve) => (decide = resolve));
}

function settle(leave) {
  decide?.(leave);
  decide = null;
}

watch(leaving, (open) => !open && settle(false));
onBeforeRouteLeave(confirmLeave);
onBeforeRouteUpdate(confirmLeave);

// Closing or reloading the tab gets the browser's own warning.
function warnUnload(event) {
  if (dirty.value) event.preventDefault();
}
window.addEventListener("beforeunload", warnUnload);
onUnmounted(() => window.removeEventListener("beforeunload", warnUnload));

watch(tab, load, { immediate: true });
watch(tab, (value) => value === "app" && loadCertificates(), { immediate: true });
watch(tab, (value) => value === "auth" && loadGroups(), { immediate: true });
watch(tab, (value) => value === "backups" && loadBackups(), { immediate: true });
watch(
  () => session.profile?.email,
  (email) => (testAddress.value ||= email ?? ""),
  { immediate: true },
);
</script>

<template>
  <PageHeader :title="section.label" :description="section.description" />

  <form v-if="settings" class="card" @submit.prevent="save">
    <!-- General -->
    <div v-if="tab === 'app'" class="p-6">
      <SettingRow v-slot="f" label="Organization name" description="Shown in the header and on the sign-in page." :error="errors.app_organization">
        <input :id="f.id" v-model="settings.app_organization" type="text" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>

      <SettingRow v-slot="f" label="Hostname" description="The address people use to reach this page." :error="errors.app_hostname">
        <input :id="f.id" v-model="settings.app_hostname" type="text" placeholder="vpn.example.com" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>

      <SettingRow v-slot="f" label="HTTP port" :error="errors.app_http_port">
        <input :id="f.id" v-model="settings.app_http_port" type="text" inputmode="numeric" class="input w-28 font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>

      <SettingRow v-slot="f" label="HTTPS port" description="Changing either port restarts the web server; reload this page at the new address afterwards." :error="errors.app_https_port">
        <input :id="f.id" v-model="settings.app_https_port" type="text" inputmode="numeric" class="input w-28 font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>

      <SettingRow
        label="Use Let's Encrypt"
        description="A free certificate, trusted by every browser and renewed automatically. The hostname must be a DNS name pointing at this server, with port 80 or 443 reachable from the internet."
      >
        <ToggleSwitch v-model="letsEncrypt" label="Use Let's Encrypt" class="sm:mt-2" />
      </SettingRow>

      <SettingRow
        v-if="letsEncrypt"
        v-slot="f"
        label="Contact email"
        description="Optional. Let's Encrypt writes here if a certificate is about to expire unrenewed."
        :error="errors.app_acme_email"
      >
        <input :id="f.id" v-model="settings.app_acme_email" type="email" placeholder="admin@example.com" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>

      <template v-else>
        <SettingRow v-slot="f" label="TLS certificate" description="Paste a PEM certificate to replace the current one. Leave blank to keep it." :error="errors.app_ssl_crt">
          <textarea :id="f.id" v-model="settings.app_ssl_crt" rows="4" placeholder="-----BEGIN CERTIFICATE-----" class="input font-mono text-xs" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>

        <SettingRow v-slot="f" label="TLS private key" description="Required with a new certificate." :error="errors.app_ssl_key">
          <textarea :id="f.id" v-model="settings.app_ssl_key" rows="4" placeholder="-----BEGIN PRIVATE KEY-----" class="input font-mono text-xs" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>
      </template>

      <SettingRow v-if="certificates.length" label="Certificates" description="The ones this server depends on, and when they expire.">
        <div class="w-full overflow-hidden rounded-[10px] border border-hair">
          <div
            v-for="certificate in certificates"
            :key="certificate.name"
            class="flex min-h-[50px] items-center justify-between gap-3 border-b border-hair px-4 py-2 last:border-0"
          >
            <span>{{ certificate.label }}</span>
            <span class="flex items-center gap-3">
              <span class="text-[13px]" :class="certificate.expires_soon ? 'font-semibold text-warn' : 'text-ink-3'">
                {{ shortDate(certificate.not_after) }}
              </span>
              <AppButton v-if="certificate.name === 'openvpn'" size="sm" @click="renewing = true">Renew</AppButton>
            </span>
          </div>
        </div>
      </SettingRow>

      <SettingRow v-slot="f" label="Keep the audit log for" description="Older events and connection history are removed once a day." :error="errors.app_event_retention_days">
        <select :id="f.id" v-model="settings.app_event_retention_days" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="30">30 days</option>
          <option value="90">90 days</option>
          <option value="180">180 days</option>
          <option value="365">1 year</option>
          <option value="0">Forever</option>
        </select>
      </SettingRow>

      <SettingRow label="Logo" description="Shown beside your organization's name on the sign-in page and in the header. PNG, JPEG, WebP or SVG, under 256 KB." :error="logoError || errors.app_logo">
        <div class="flex items-center gap-3">
          <img v-if="settings.app_logo" :src="settings.app_logo" alt="" class="h-10 max-w-[160px] rounded-md border border-hair bg-white object-contain p-1" />
          <label class="inline-flex h-9 cursor-pointer items-center rounded-lg border border-field bg-surface px-3 font-medium text-ink-2 hover:bg-subtle">
            {{ settings.app_logo ? "Replace" : "Upload" }}
            <input type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" class="sr-only" @change="chooseLogo" />
          </label>
          <AppButton v-if="settings.app_logo" size="sm" @click="settings.app_logo = ''">Remove</AppButton>
        </div>
      </SettingRow>

      <SettingRow v-slot="f" label="Sign-in notice" description="Shown under the sign-in form, such as an authorized-use warning." :error="errors.app_signin_notice">
        <textarea :id="f.id" v-model="settings.app_signin_notice" rows="3" maxlength="1000" placeholder="This system is for authorized use only." class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Help contact" description="An email address or https link, shown as “Need help? Contact IT” on the sign-in page. Leave empty to hide it." :error="errors.app_support_contact">
        <input :id="f.id" v-model="settings.app_support_contact" type="text" placeholder="it@example.com" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
    </div>

    <!-- Single sign-on -->
    <div v-else-if="tab === 'auth'" class="p-6">
      <SettingRow v-slot="f" label="Provider" description="OpenID Connect works with Okta, Microsoft Entra ID, Authentik, Keycloak and most other identity providers." :error="errors.oauth2_provider">
        <select :id="f.id" v-model="settings.oauth2_provider" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="none">Disabled</option>
          <option value="google">Google</option>
          <option value="oidc">OpenID Connect</option>
        </select>
      </SettingRow>

      <template v-if="settings.oauth2_provider !== 'none'">
        <SettingRow label="Redirect URI" description="Register this with the provider.">
          <code class="w-full rounded-lg border border-hair bg-subtle px-3 py-2.5 font-mono text-[13px] break-all">{{ settings.oauth2_redirect_uri }}</code>
        </SettingRow>

        <template v-if="settings.oauth2_provider === 'oidc'">
          <SettingRow v-slot="f" label="Provider name" description="Shown on the sign-in button, as in “Continue with Okta”." :error="errors.oauth2_name">
            <input :id="f.id" v-model="settings.oauth2_name" type="text" maxlength="40" placeholder="Okta" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
          </SettingRow>
          <SettingRow
            v-slot="f"
            label="Issuer URL"
            description="Where the provider's settings are discovered from, such as https://example.okta.com or https://login.microsoftonline.com/<tenant>/v2.0."
            :error="errors.oauth2_issuer"
          >
            <input :id="f.id" v-model="settings.oauth2_issuer" type="url" placeholder="https://" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
          </SettingRow>
        </template>

        <SettingRow v-slot="f" label="Client ID" :error="errors.oauth2_client_id">
          <input :id="f.id" v-model="settings.oauth2_client_id" type="text" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>
        <SettingRow v-slot="f" label="Client secret" :error="errors.oauth2_client_secret">
          <input :id="f.id" v-model="settings.oauth2_client_secret" type="password" autocomplete="off" :placeholder="settings.oauth2_client_secret_set ? 'Saved. Leave blank to keep it.' : ''" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>
        <SettingRow
          v-slot="f"
          label="Only accounts from this domain"
          :description="
            settings.oauth2_provider === 'google'
              ? 'Your Google Workspace domain. Leave blank to accept any Google account whose email matches a user here.'
              : 'Leave blank to accept any address the provider has verified. Set it for providers such as Microsoft Entra ID that don\'t say whether an address is verified.'
          "
          :error="errors.oauth2_allowed_domain"
        >
          <input :id="f.id" v-model="settings.oauth2_allowed_domain" type="text" placeholder="example.com" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>

        <SettingRow
          v-slot="f"
          label="Create accounts on first sign-in"
          description="Anyone from the domain above who signs in through the provider without an account gets one, in this group."
          :error="errors.oauth2_auto_group"
        >
          <select :id="f.id" v-model="settings.oauth2_auto_group" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
            <option value="">Don't create accounts</option>
            <option v-for="group in groups" :key="group.id" :value="group.id">In {{ group.name }}</option>
          </select>
        </SettingRow>

        <SettingRow
          label="Single sign-on only"
          description="Turns off passwords for everyone but administrators, who keep theirs so a broken provider can't lock everyone out."
          :error="errors.oauth2_only"
        >
          <ToggleSwitch v-model="ssoOnly" label="Single sign-on only" class="sm:mt-2" />
        </SettingRow>
      </template>
    </div>

    <!-- Email -->
    <div v-else-if="tab === 'mail'" class="p-6">
      <SettingRow v-slot="f" label="SMTP server" :error="errors.smtp_host">
        <input :id="f.id" v-model="settings.smtp_host" type="text" placeholder="smtp.example.com" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Port" :error="errors.smtp_port">
        <input :id="f.id" v-model="settings.smtp_port" type="text" inputmode="numeric" class="input w-28 font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Username" :error="errors.smtp_username">
        <input :id="f.id" v-model="settings.smtp_username" type="text" autocomplete="off" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Password" :error="errors.smtp_password">
        <input :id="f.id" v-model="settings.smtp_password" type="password" autocomplete="new-password" :placeholder="settings.smtp_password_set ? 'Saved. Leave blank to keep it.' : ''" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Reply-to address" description="Where replies to the application's emails go." :error="errors.smtp_reply_address">
        <input :id="f.id" v-model="settings.smtp_reply_address" type="email" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Send a test email" description="Save first; the test uses the saved settings." :error="testError">
        <div class="flex w-full gap-2">
          <input :id="f.id" v-model="testAddress" type="email" class="input min-w-0 flex-1" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
          <AppButton :disabled="testing || !testAddress" @click="sendTest">
            {{ testing ? "Sending…" : "Send test" }}
          </AppButton>
        </div>
      </SettingRow>
    </div>

    <!-- Alerts -->
    <div v-else-if="tab === 'alerts'" class="p-6">
      <SettingRow v-slot="f" label="Email alerts to" description="One address per line. Sent with the email settings." :error="errors.alert_emails">
        <textarea :id="f.id" v-model="settings.alert_emails" rows="3" placeholder="ops@example.com" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Webhook" description="A Slack, Microsoft Teams, Google Chat, Mattermost or Discord incoming webhook URL." :error="errors.alert_webhook_url">
        <input :id="f.id" v-model="settings.alert_webhook_url" type="password" autocomplete="off" :placeholder="settings.alert_webhook_url_set ? 'Saved. Leave blank to keep it.' : 'https://hooks.slack.com/services/…'" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow label="When OpenVPN stops" description="Checked every minute. Stopping it yourself from here isn't alerted.">
        <ToggleSwitch v-model="alertVPNDown" label="Alert when OpenVPN stops" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="When a certificate is about to expire" description="Daily, from 30 days before the certificate authority's, the OpenVPN server's or the web server's certificate expires.">
        <ToggleSwitch v-model="alertCertificates" label="Alert when a certificate is about to expire" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="When an account is locked out" description="After 10 wrong passwords or codes in a row, which can mean someone is guessing.">
        <ToggleSwitch v-model="alertLockouts" label="Alert when an account is locked out" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="When a device is added" description="Through the web page or OpenVPN Connect.">
        <ToggleSwitch v-model="alertNewDevice" label="Alert when a device is added" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="When staff sign in" description="Administrators and help desk signing in to this page.">
        <ToggleSwitch v-model="alertAdminSignIn" label="Alert when staff sign in" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="When someone signs in from a new address" description="An address that person hasn't signed in from before.">
        <ToggleSwitch v-model="alertNewAddress" label="Alert on sign-ins from new addresses" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="Daily summary" description="Sign-ins, connections, data and changes from the last 24 hours, each morning after 8.">
        <ToggleSwitch v-model="alertDailyDigest" label="Send a daily summary" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="Send a test alert" description="Save first; the test goes to the saved addresses and webhook." :error="alertTestError">
        <AppButton :disabled="alertTesting" @click="sendTestAlert">
          {{ alertTesting ? "Sending…" : "Send test alert" }}
        </AppButton>
      </SettingRow>
    </div>

    <!-- Security -->
    <div v-else-if="tab === 'security'" class="p-6">
      <SettingRow v-slot="f" label="Lock an account after" description="Wrong passwords or codes in a row, on the web or the VPN." :error="errors.auth_lockout_attempts">
        <select :id="f.id" v-model="settings.auth_lockout_attempts" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="5">5 attempts</option>
          <option value="10">10 attempts</option>
          <option value="20">20 attempts</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="Keep it locked for" description="An administrator can unlock it sooner from the user's page." :error="errors.auth_lockout_minutes">
        <select :id="f.id" v-model="settings.auth_lockout_minutes" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="5">5 minutes</option>
          <option value="15">15 minutes</option>
          <option value="30">30 minutes</option>
          <option value="60">1 hour</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="Sign out when idle for" description="Changing either session length restarts the web server." :error="errors.auth_session_idle_minutes">
        <select :id="f.id" v-model="settings.auth_session_idle_minutes" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="15">15 minutes</option>
          <option value="30">30 minutes</option>
          <option value="60">1 hour</option>
          <option value="240">4 hours</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="Sign out after" description="However active the session is." :error="errors.auth_session_hours">
        <select :id="f.id" v-model="settings.auth_session_hours" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="8">8 hours</option>
          <option value="12">12 hours</option>
          <option value="24">24 hours</option>
          <option value="72">3 days</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="Passwords at least" description="Long passphrases are stronger than short complex passwords." :error="errors.auth_password_min_length">
        <select :id="f.id" v-model="settings.auth_password_min_length" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="8">8 characters</option>
          <option value="10">10 characters</option>
          <option value="12">12 characters</option>
          <option value="14">14 characters</option>
          <option value="16">16 characters</option>
        </select>
      </SettingRow>
      <SettingRow label="Require mixed characters" description="An uppercase letter, a lowercase letter and a digit.">
        <ToggleSwitch v-model="passwordComplexity" label="Require mixed characters" class="sm:mt-2" />
      </SettingRow>
      <SettingRow
        v-slot="f"
        label="Administration networks"
        description="Only allow the administration pages from these addresses or networks, one per line. Leave blank to allow them from anywhere."
        :error="errors.auth_admin_networks"
      >
        <textarea :id="f.id" v-model="settings.auth_admin_networks" rows="3" placeholder="10.0.0.0/8" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
    </div>

    <!-- Backups -->
    <div v-else-if="tab === 'backups'" class="p-6">
      <SettingRow label="Latest backup" :description="backups?.last_error ? 'The last attempt failed.' : 'Taken every night, and now if you ask.'">
        <div class="flex w-full flex-col gap-2">
          <div class="flex items-center gap-3">
            <span v-if="backups?.last" class="text-ink-2" :title="dateTime(backups.last)">{{ relative(backups.last) }}</span>
            <span v-else class="text-ink-4">None yet</span>
            <AppButton size="sm" :disabled="backingUp" @click="backUpNow">{{ backingUp ? "Backing up…" : "Back up now" }}</AppButton>
          </div>
          <p v-if="backups?.last_error" class="field-error">{{ backups.last_error }}</p>
        </div>
      </SettingRow>
      <SettingRow v-slot="f" label="Keep" description="Older backups are removed, here and in the bucket." :error="errors.backup_keep">
        <select :id="f.id" v-model="settings.backup_keep" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="3">The last 3</option>
          <option value="7">The last 7</option>
          <option value="14">The last 14</option>
          <option value="30">The last 30</option>
        </select>
      </SettingRow>
      <SettingRow v-if="backups?.files.length" label="On this machine" description="They hold the secret key, so keep downloaded copies safe.">
        <div class="w-full overflow-hidden rounded-[10px] border border-hair">
          <div v-for="file in backups.files" :key="file.name" class="flex min-h-[50px] items-center justify-between gap-3 border-b border-hair px-4 py-2 last:border-0">
            <span class="text-[13px]" :title="file.name">{{ dateTime(file.created) }} <span class="text-ink-4">· {{ bytes(file.size) }}</span></span>
            <a :href="`/api/admin/backups/${file.name}`" class="text-[13px] font-medium text-accent no-underline hover:text-accent-strong">Download</a>
          </div>
        </div>
      </SettingRow>
      <SettingRow v-slot="f" label="S3 endpoint" description="Copies each backup to an S3-compatible bucket: Amazon S3, Backblaze B2, Cloudflare R2, Wasabi or MinIO. Leave blank to keep backups on this machine only." :error="errors.backup_s3_endpoint">
        <input :id="f.id" v-model="settings.backup_s3_endpoint" type="url" placeholder="https://s3.us-east-1.amazonaws.com" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <template v-if="settings.backup_s3_endpoint">
        <SettingRow v-slot="f" label="Bucket" :error="errors.backup_s3_bucket">
          <input :id="f.id" v-model="settings.backup_s3_bucket" type="text" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>
        <SettingRow v-slot="f" label="Region" description="Some providers need it, such as us-east-1." :error="errors.backup_s3_region">
          <input :id="f.id" v-model="settings.backup_s3_region" type="text" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>
        <SettingRow v-slot="f" label="Folder" description="Optional. Backups go under this prefix in the bucket." :error="errors.backup_s3_prefix">
          <input :id="f.id" v-model="settings.backup_s3_prefix" type="text" placeholder="mangle-vpn" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>
        <SettingRow v-slot="f" label="Access key" :error="errors.backup_s3_access_key">
          <input :id="f.id" v-model="settings.backup_s3_access_key" type="text" autocomplete="off" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>
        <SettingRow v-slot="f" label="Secret key" :error="errors.backup_s3_secret_key">
          <input :id="f.id" v-model="settings.backup_s3_secret_key" type="password" autocomplete="off" :placeholder="settings.backup_s3_secret_key_set ? 'Saved. Leave blank to keep it.' : ''" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
        </SettingRow>
      </template>
    </div>

    <!-- OpenVPN -->
    <div v-else-if="tab === 'vpn'" class="p-6">
      <SettingRow v-slot="f" label="Hostname" description="The address devices connect to." :error="errors.vpn_hostname">
        <input :id="f.id" v-model="settings.vpn_hostname" type="text" placeholder="vpn.example.com" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Port" :error="errors.vpn_port">
        <input :id="f.id" v-model="settings.vpn_port" type="text" inputmode="numeric" class="input w-28 font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Protocol" description="UDP is faster; TCP gets through stricter networks." :error="errors.vpn_protocol">
        <select :id="f.id" v-model="settings.vpn_protocol" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="udp">UDP</option>
          <option value="tcp">TCP</option>
        </select>
      </SettingRow>
      <SettingRow
        label="Kernel acceleration"
        description="Data channel offload (DCO) lets the kernel encrypt VPN traffic instead of OpenVPN, for much higher speeds at lower CPU."
      >
        <div class="flex flex-col gap-1 sm:pt-2">
          <StatusDot v-if="session.vpnOffload === true" state="up" label="On" />
          <template v-else-if="session.vpnOffload === false">
            <StatusDot state="off" label="Off" />
            <p class="text-[13px] leading-relaxed text-ink-4">
              Install the DCO kernel module on the server (<code class="font-mono text-[12px]">openvpn-dco-dkms</code>
              on Debian and Ubuntu, with OpenVPN 2.6 or newer), then restart OpenVPN.
            </p>
          </template>
          <span v-else class="text-[13px] text-ink-4">Shown while OpenVPN is running.</span>
        </div>
      </SettingRow>
      <SettingRow v-slot="f" label="Listen interface" :error="errors.vpn_interface">
        <select :id="f.id" v-model="settings.vpn_interface" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option v-for="(address, name) in interfaces" :key="name" :value="name">{{ name }} · {{ address || "no address" }}</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="NAT interface" description="Where client traffic leaves the server." :error="errors.vpn_nat_interface">
        <select :id="f.id" v-model="settings.vpn_nat_interface" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option v-for="(address, name) in interfaces" :key="name" :value="name">{{ name }} · {{ address || "no address" }}</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="Client subnet" description="The lower half is handed out as devices connect; the upper half is kept for fixed addresses." :error="errors.vpn_subnet">
        <input :id="f.id" v-model="settings.vpn_subnet" type="text" placeholder="10.8.0.0/24" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow label="Send all traffic through the VPN" description="When off, only the routes below go through the VPN.">
        <ToggleSwitch v-model="redirectGateway" label="Send all traffic through the VPN" class="sm:mt-2" />
      </SettingRow>
      <SettingRow v-slot="f" label="Routes" description="One CIDR network per line." :error="errors.vpn_routes">
        <textarea :id="f.id" v-model="settings.vpn_routes" rows="3" placeholder="10.0.0.0/16" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="DNS servers" description="One address per line." :error="errors.vpn_nameservers">
        <textarea :id="f.id" v-model="settings.vpn_nameservers" rows="3" placeholder="10.0.0.2" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Search domain" :error="errors.vpn_domain">
        <input :id="f.id" v-model="settings.vpn_domain" type="text" placeholder="corp.example.com" class="input font-mono text-[13px]" :aria-invalid="f.invalid" :aria-describedby="f.describedby" />
      </SettingRow>
      <SettingRow v-slot="f" label="Ask for a new code" description="After this long, a connection asks for a fresh authenticator code." :error="errors.vpn_session_hours">
        <select :id="f.id" v-model="settings.vpn_session_hours" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="0">Never, once per connection</option>
          <option value="8">Every 8 hours</option>
          <option value="12">Every 12 hours</option>
          <option value="24">Every 24 hours</option>
          <option value="168">Every 7 days</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="Disconnect when idle" description="Applies to profiles issued from now on." :error="errors.vpn_idle_minutes">
        <select :id="f.id" v-model="settings.vpn_idle_minutes" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="0">Never</option>
          <option value="30">After 30 minutes</option>
          <option value="60">After 1 hour</option>
          <option value="240">After 4 hours</option>
          <option value="480">After 8 hours</option>
        </select>
      </SettingRow>
      <SettingRow
        label="Ask for the account password too"
        description="Connecting asks for the account password and the authenticator code, in separate fields. Devices added before this is turned on must be added again."
      >
        <ToggleSwitch v-model="requirePassword" label="Ask for the account password too" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="Let devices reach each other" description="Any connected device can reach any other, whatever its group's firewall rules. When off, group rules decide.">
        <ToggleSwitch v-model="deviceToDevice" label="Let devices reach each other" class="sm:mt-2" />
      </SettingRow>
      <SettingRow label="One connection per person" description="Connecting from a second device disconnects the first.">
        <ToggleSwitch v-model="singleSession" label="One connection per person" class="sm:mt-2" />
      </SettingRow>
      <SettingRow
        label="Share port 443 with the web server"
        description="OpenVPN takes TCP 443 and hands everything else to the web server, so the VPN gets through networks that only allow web traffic. Needs TCP, port 443, and the web server on another HTTPS port."
        :error="errors.vpn_port_share"
      >
        <ToggleSwitch v-model="portShare" label="Share port 443 with the web server" class="sm:mt-2" />
      </SettingRow>
      <SettingRow v-slot="f" label="Packet size" description="Lower it if connections work but some sites hang, as on some mobile and DSL links. UDP only; applies fully to profiles issued from now on." :error="errors.vpn_mssfix">
        <select :id="f.id" v-model="settings.vpn_mssfix" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="0">OpenVPN's default</option>
          <option value="1400">1400</option>
          <option value="1360">1360</option>
          <option value="1300">1300</option>
          <option value="1200">1200</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="Device certificates last" description="Applies to devices added from now on. An expired device stops connecting and has to be added again." :error="errors.pki_device_days">
        <select :id="f.id" v-model="settings.pki_device_days" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="0">As long as the certificate authority</option>
          <option value="90">90 days</option>
          <option value="180">180 days</option>
          <option value="365">1 year</option>
          <option value="730">2 years</option>
        </select>
      </SettingRow>
      <SettingRow v-slot="f" label="Log detail" description="What OpenVPN writes to its log, shown under Logs." :error="errors.vpn_log_level">
        <select :id="f.id" v-model="settings.vpn_log_level" class="input" :aria-invalid="f.invalid" :aria-describedby="f.describedby">
          <option value="1">Quiet: errors only</option>
          <option value="3">Normal</option>
          <option value="4">Detailed: for troubleshooting</option>
        </select>
      </SettingRow>
    </div>

    <!-- Stays in view while there is something to save, however far down
         the form the edit was made. -->
    <div
      class="sticky bottom-0 flex items-center justify-between gap-3 rounded-b-xl border-t border-hair px-6 py-3.5 transition-colors"
      :class="dirty ? 'bg-accent-soft' : 'bg-surface'"
    >
      <span v-if="dirty" class="text-[13px] font-medium text-accent-strong">You have unsaved changes.</span>
      <span v-else-if="tab === 'vpn'" class="text-[13px] text-ink-4">Changes apply after an OpenVPN restart.</span>
      <span v-else />
      <div class="flex items-center gap-2">
        <AppButton v-if="dirty" :disabled="saving" @click="discard">Discard</AppButton>
        <AppButton variant="primary" type="submit" :disabled="saving || !dirty">
          {{ saving ? "Saving…" : "Save changes" }}
        </AppButton>
      </div>
    </div>
  </form>

  <ConfirmDialog
    v-model:open="renewing"
    title="Renew the OpenVPN server certificate?"
    message="The server gets a new certificate from the same certificate authority, so devices keep working without new profiles. It takes effect when OpenVPN is restarted, which disconnects everyone briefly."
    confirm-label="Renew certificate"
    :danger="false"
    :action="renewVPNCertificate"
  />

  <ConfirmDialog
    v-model:open="leaving"
    title="Discard unsaved changes?"
    :message="`Your changes to ${section.label} settings haven't been saved.`"
    confirm-label="Discard changes"
    :action="() => settle(true)"
  />
</template>
