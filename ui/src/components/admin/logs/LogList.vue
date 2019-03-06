<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="seven wide column">
        <h1>Logs</h1>
      </div>
    </div><!-- #Header -->

    <p>
      Logs are displayed in real-time and refreshed every 5 seconds.
    </p>

    <!-- #Tabs -->
    <div class="ui pointing secondary menu">
      <a class="active item" data-tab="application">
        <i class="code icon left"></i> Application
      </a>
      <a class="item" data-tab="openvpn">
        <i class="cloud icon left"></i> OpenVPN
      </a>
      <a class="item" data-tab="iptables">
        <i class="fire icon left"></i> IPTables
      </a>
    </div><!-- #Tabs -->

    <!-- #Application -->
    <div class="ui active tab" data-tab="application">
      <pre id="appLog" class="log">{{ appLog }}</pre>
    </div><!-- #Application -->

    <!-- #OpenVPN -->
    <div class="ui tab" data-tab="openvpn">
      <pre id="openvpnLog" class="log">{{ openvpnLog }}</pre>
    </div><!-- #OpenVPN -->

    <!-- #IPTables -->
    <div class="ui tab" data-tab="iptables">
      <pre id="iptablesLog" class="log">{{ iptablesLog }}</pre>
    </div><!-- #IPTables -->
  </div>
</template>

<script>
  import mixins from "@/utils/mixins"

  export default {
    name: "LogList",
    mixins: [mixins.AppComponentMixin,],
    data() {
      return {
        appLog: "",
        appOffset: 0,
        appRefreshInterval: null,
        iptablesLog: "",
        iptablesRefreshInterval: null,
        openvpnLog: "",
        openvpnOffset: 0,
        openvpnRefreshInterval: null,
      }
    },
    destroyed() {
      this.clearIntervals();
    },
    mounted() {
      this.getApplicationLog();
      this.appRefreshInterval = setInterval(this.getApplicationLog, 5000);

      // Event handlers for properly refreshing the OpenVPN log when the OpenVPN
      // server status changes
      this.store.events.$on("vpnStatusChange", this.onVPNStatusChange);
      this.store.events.$on("vpnRestart", this.onVPNStatusChange);

      // Reloads the data for the active tab
      $(".secondary.menu .item").tab({
        onLoad: (tabPath, parameterArray, historyEvent) => {
          this.clearIntervals();
          switch (tabPath) {
            case "application":
              this.getApplicationLog();
              this.appRefreshInterval = setInterval(this.getApplicationLog, 5000);
              break;
            case "openvpn":
              this.getOpenvpnLog();
              this.openvpnRefreshInterval = setInterval(this.getOpenvpnLog, 5000);
              break;
            case "iptables":
              this.getIptablesLog();
              this.iptablesRefreshInterval = setInterval(this.getIptablesLog, 5000);
              break;
          }
        },
      });
    },
    methods: {
      // getApplicationLog retrieves the Django application log.
      getApplicationLog() {
        this.axios.get(`/api/admin/logs/app/?offset=${this.appOffset}`).then(resp => {
          this.appLog += resp.data.data;
          this.appOffset = resp.data.offset;
        });
      },

      // getIptablesLog retrieves and sets the system's iptables rules.
      getIptablesLog() {
        this.axios.get(`/api/admin/logs/iptables/`).then(resp => {
          this.iptablesLog = resp.data.data;
        });
      },

      // getOpenvpnLog retrieves and appends the latest data from the OpenVPN log.
      getOpenvpnLog() {
        this.axios.get(`/api/admin/logs/openvpn/?offset=${this.openvpnOffset}`).then(resp => {
          this.openvpnLog += resp.data.data;
          this.openvpnOffset = resp.data.offset;
        });
      },

      // clearIntervals clears all refresh intervals.
      clearIntervals() {
        clearInterval(this.openvpnRefreshInterval);
        clearInterval(this.appRefreshInterval);
        clearInterval(this.iptablesRefreshInterval);
      },

      // onVPNStatusChange resets the OpenVPN log.
      onVPNStatusChange(isRunning) {
        this.openvpnOffset = 0;
        this.openvpnLog = "";
      },
    },
  }
</script>

<style>
  .log {
    background: #f6f6f6;
    font-family: "Menlo", "Consolas", sans-serif;
    font-size: 0.85em;
    height: 650px;
    overflow-y: scroll;
    padding: 1em;
  }
</style>
