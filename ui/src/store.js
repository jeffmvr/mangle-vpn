import Vue from 'vue';

export default {
  activeAdminPage: "",
  activePage: "",
  appInitialized: false,
  appOrganization: "Mangle",
  appVersion: "",
  updateAvailable: false,
  vpnRestartPending: false,
  profile: {},
  events: new Vue(),
}
