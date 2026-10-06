import { defineStore } from "pinia";
import { computed, ref } from "vue";

import api from "@/lib/api";

/**
 * Everything about who is signed in and what the server is doing.
 *
 * The profile is loaded once at start up and kept here rather than fetched
 * per view, because almost every screen needs the user's group or their
 * administrator flag to decide what to show. The OpenVPN state lives here
 * too, because the sidebar, the header and the settings page all act on it.
 */
export const useSession = defineStore("session", () => {
  const profile = ref(null);
  const organization = ref("Mangle");
  const logoUrl = ref("");
  const version = ref("");
  const ready = ref(false);

  const vpnRunning = ref(false);
  // False until the first status read, so a screen does not report OpenVPN
  // as stopped while it is still asking.
  const vpnKnown = ref(false);
  // Whether OpenVPN's data channel offload is in use: true, false, or null
  // while there is no running tunnel to tell from.
  const vpnOffload = ref(null);
  const vpnBusy = ref(false);
  const restartPending = ref(false);

  const isAdmin = computed(() => profile.value?.is_admin === true);
  // The help desk shares the administration pages, read-only apart from
  // helping people back in; isAdmin still guards everything else.
  const isStaff = computed(() => isAdmin.value || profile.value?.role === "helpdesk");
  // Staff outside the networks administration is limited to see none of it.
  const canAdminister = computed(() => isStaff.value && profile.value?.admin_reachable !== false);
  const deviceLimit = computed(() => profile.value?.group?.max_devices ?? 0);
  const deviceCount = computed(() => profile.value?.devices?.length ?? 0);
  const canAddDevice = computed(() => deviceCount.value < deviceLimit.value);

  /** Loads the public details the header needs. */
  async function loadInfo() {
    const { data } = await api.get("/info");
    organization.value = data.app_organization;
    logoUrl.value = data.logo_url ?? "";
    version.value = data.app_version;
    restartPending.value = data.vpn_restart_pending;
  }

  /** Loads the signed in user's own account. */
  async function loadProfile() {
    const { data } = await api.get("/profile");
    profile.value = data;
  }

  /**
   * Loads everything the shell needs before the first view renders. Both
   * the application and the router's admin guard ask for this, so it runs
   * once and every caller waits on the same load.
   */
  let starting = null;
  function start() {
    starting ??= Promise.all([loadInfo(), loadProfile()]).then(() => {
      ready.value = true;
    });
    return starting;
  }

  /** Copies an OpenVPN status response into the store. */
  function applyStatus(data) {
    vpnRunning.value = data.status;
    vpnKnown.value = true;
    vpnOffload.value = data.offload ?? null;
    restartPending.value = data.restart_pending;
  }

  /** Reads whether OpenVPN is up and whether it owes a restart. */
  async function refreshVpn() {
    const { data } = await api.get("/admin/openvpn");
    applyStatus(data);
  }

  /** Runs one OpenVPN control action, one at a time. */
  async function controlVpn(action) {
    if (vpnBusy.value) return;
    vpnBusy.value = true;
    try {
      const { data } = await api.post(`/admin/openvpn/${action}`);
      applyStatus(data);
    } finally {
      vpnBusy.value = false;
    }
  }

  return {
    profile,
    organization,
    logoUrl,
    version,
    ready,
    vpnRunning,
    vpnKnown,
    vpnOffload,
    vpnBusy,
    restartPending,
    isAdmin,
    isStaff,
    canAdminister,
    deviceLimit,
    deviceCount,
    canAddDevice,
    loadInfo,
    loadProfile,
    start,
    refreshVpn,
    toggleVpn: () => controlVpn("toggle"),
    restartVpn: () => controlVpn("restart"),
  };
});
