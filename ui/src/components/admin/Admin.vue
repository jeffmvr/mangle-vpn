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
        <a href="javascript:" :class="['item', 'teal', {disabled: store.updateAvailable === false}]" @click="update">
          <i class="cloud download icon left"></i>
          Update
        </a>
      </div><!-- #Side Nav -->

      <div class="item" style="padding-top: 1em; text-align: center; width: 100%;">
        <p>
          OpenVPN server is &nbsp;
          <span class="ui green horizontal label" v-if="vpnStatus === true">Running</span>
          <span class="ui red horizontal label" v-else>Stopped</span>
        </p>

        <div class="ui icon buttons">
          <button :class="['ui', 'button', {disabled: vpnStatus}]" @click="toggleOpenVPN()">
            <i class="play green icon"></i>
          </button>
          <button :class="['ui', 'button', {disabled: !vpnStatus}]" @click="toggleOpenVPN()">
            <i class="stop red icon"></i>
          </button>
          <button :class="['ui', 'button', {disabled: !vpnStatus}]" @click="restartOpenVPN()">
            <i class="sync alternate icon"></i>
          </button>
        </div>
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
      /**
       * Sets the current Admin page based on the route.
       * @returns {null}
       */
      setActiveAdminPage() {
        this.store.activeAdminPage = this.$router.currentRoute.path.split("/")[2];
      },

      /**
       * Retrieves and sets the OpenVPN server status.
       * @returns {null}
       */
      getOpenVPNStatus() {
        this.axios.get("/admin/openvpn/").then(resp => {
          this.vpnStatus = resp.data.status;
        });
      },

      /**
       * Starts or stops the OpenVPN server based on it's current status.
       * @returns {null}
       */
      toggleOpenVPN() {
        this.axios.get("/admin/openvpn/toggle/").then(resp => {
          this.vpnStatus = resp.data.status;
        });

        if (this.vpnStatus === true) {
          this.toastr.success("Stopping the OpenVPN server.", "Stopping OpenVPN");
        } else {
          this.toastr.success("Starting the OpenVPN server.", "Starting OpenVPN");
        }
      },

      /**
       * Restarts the OpenVPN server.
       * @returns {null}
       */
      restartOpenVPN() {
        this.axios.get("/admin/openvpn/restart/").then(resp => {
          this.vpnStatus = resp.data.status;
          this.store.events.$emit("vpnRestart");
        });
        this.toastr.success("Restarting the OpenVPN server.", "Restarting OpenVPN");
      },

      /**
       * Updates the application to the latest version.
       * @returns {null}
       */
      update() {
        this.axios.post("/admin/update")
          .then(resp => {
            this.toastr.success("Application updated, reloading...", "Application Updated");

            // simulate a sleep for 3s to give the web server time to restart before refreshing
            new Promise(resolve => setTimeout(resolve, 3000))
              .then(() => {
                location.reload(true);
              });
          });
      }
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
