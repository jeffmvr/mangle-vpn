<template>
  <div class="ui row">
    <!-- #Side Nav -->
    <div class="four wide column">
      <div class="ui fluid large pointing vertical menu">
        <div class="header item">
          Administration
        </div>
        <a href="/#/admin/clients" :class="['item', {'active': store.activeAdminPage === 'clients'}]">
          <i class="laptop icon left"></i>
          Clients
        </a>
        <a href="/#/admin/users" :class="['item', {'active': store.activeAdminPage === 'users'}]">
          <i class="user icon left"></i>
          Users
        </a>
        <a href="/#/admin/groups" :class="['item', {'active': store.activeAdminPage === 'groups'}]">
          <i class="users icon left"></i>
          Groups
        </a>
        <a href="/#/admin/events" :class="['item', {'active': store.activeAdminPage === 'events'}]">
          <i class="list icon left"></i>
          Events
        </a>
        <a href="/#/admin/settings" :class="['item', {'active': store.activeAdminPage === 'settings'}]">
          <i class="cog icon left"></i>
          Settings
        </a>
      </div><!-- #Side Nav -->

      <div v-if="vpnStatus === true">
        <button class="ui button red fluid" @click="toggleOpenVPN()">
          Stop OpenVPN
        </button>
        <button class="ui button fluid" style="margin-top: 0.5em;" @click="restartOpenVPN()">
          Restart OpenVPN
        </button>
      </div>
      <div v-else>
        <button class="ui button green fluid" @click="toggleOpenVPN()">
        Start OpenVPN
        </button>
      </div>
    </div>

    <!-- #AdminContent -->
    <div class="twelve wide column">
      <router-view></router-view>
    </div><!-- #AdminContent -->
  </div>
</template>


<script>
  import mixins from "@/utils/mixins"

  export default {
    name: "Admin",
    mixins: [
      mixins.AppComponentMixin,
    ],
    data() {
      return {
        vpnStatus: false,
        refreshInterval: null,
      }
    },
    mounted() {
      this.setActiveAdminPage();
      this.getOpenVPNStatus();
      this.refreshInterval = setInterval(this.getOpenVPNStatus, 5000);
    },
    destroyed() {
      clearInterval(this.refreshInterval);
    },
    methods: {
      // setActiveAdminPage sets the active Admin page based on the current route path.
      setActiveAdminPage() {
        this.store.activeAdminPage = this.$router.currentRoute.path.split("/")[2];
      },

      // getOpenVPNStatus retrieves and sets the status of the OpenVPN server.
      getOpenVPNStatus() {
        this.axios.get("/admin/openvpn/").then(resp => {
          this.vpnStatus = resp.data.status;
        });
      },

      // toggleOpenVPN starts or stops the OpenVPN server depending on its status.
      toggleOpenVPN() {
        this.axios.get("/admin/openvpn/toggle/").then(resp => {
          this.vpnStatus = resp.data.status;
        });

        if (this.vpnStatus === true) {
          this.toastr.success("Stopping the OpenVPN server.", "Stopping");
        } else {
          this.toastr.success("Starting the OpenVPN server.", "Starting");
        }
      },

      // restartOpenVPN restarts the OpenVPN server.
      restartOpenVPN() {
        this.axios.get("/admin/openvpn/restart/").then(resp => {
          this.vpnStatus = resp.data.status;
          this.store.events.$emit("vpnRestart");
        });
        this.toastr.success("Restarting the OpenVPN server.", "Restarting");
      },
    }, // #Methods
    watch: {
      // Watches for route changes and updates the active admin page accordingly.
      $route(to, from) {
        this.setActiveAdminPage();
      },

      // vpnStatus watches for changes to the OpenVPN server status and emits an
      // event for listeners.
      vpnStatus(newValue, oldValue) {
        this.store.events.$emit("vpnStatusChange", newValue);
      }
    }, // #Watch
  }
</script>
