<template>
  <div class="ui row">
    <!-- #Instructions -->
    <div class="ten wide column">
      <h1>Getting Started</h1>
      <p>
        Generate and download an OpenVPN configuration file for each of your devices and follow the instructions below
        for your device's operating system.
      </p>
      <p>
        In addition to the desktop clients, the OpenVPN Connect mobile application is available from both the
        <a href="https://play.google.com/store/apps/details?id=net.openvpn.openvpn">Android Play Store</a> and
        <a href="https://itunes.apple.com/us/app/openvpn-connect/id590379981?mt=8">Apple App Store</a>.
      </p>

      <!-- #macOS -->
      <h3><i class="apple icon"></i>Apple macOS</h3>
      <div class="ui divider"></div>
      <ol>
        <li>
          Download and install the <a href="https://tunnelblick.net/release/Tunnelblick_3.7.6a_build_5080.dmg">
          Tunnelblick</a> client
        </li>
        <li>Download and double click a device configuration file (.ovpn) to import it</li>
        <li>Click the Tunnelblick application icon on the macOS status bar and click Connect</li>
        <li>Enter your e-mail address (username) and two-factor authentication code (password) when prompted</li>
      </ol><!-- #macOS -->

      <!-- #Windows -->
      <h3><i class="windows icon"></i>Microsoft Windows</h3>
      <div class="ui divider"></div>
      <ol>
        <li>
          Download and install the latest <a href="https://openvpn.net/index.php/open-source/downloads.html">
          official OpenVPN client</a>
        </li>
        <li>
          Right click the OpenVPN GUI icon in the Windows system tray and click <i>Import File...</i> to import
          the device configuration (.ovpn) file
        </li>
        <li>Right click the OpenVPN GUI icon in the Windows status tray and select Connect</li>
        <li>Enter your e-mail address (username) and two-factor authentication code (password) when prompted</li>
      </ol><!-- #Windows -->

      <!-- #Linux -->
      <h3><i class="linux icon"></i>Ubuntu Linux</h3>
      <div class="ui divider"></div>
      <p>
        Download and run the <a href="/scripts/openvpn-client-installer.sh">OpenVPN client installation script</a> which
        will handle installation of OpenVPN and additional required packages.
      </p>
      <div class="ui message console">
        $ sudo ./openvpn-client-installer.sh
      </div><!-- #Linux -->
      <p>
        Once OpenVPN has been installed, you can connect to the OpenVPN server using the following command. Be sure to
        enter your e-mail address (username) and two-factor authentication code (password) when prompted:
      </p>
      <div class="ui message console">
        $ sudo openvpn --config /path/to/config.ovpn
      </div><!-- #Linux -->
    </div><!-- #Instructions -->

    <!-- #Devices -->
    <div class="six wide column">
      <div class="ui fluid vertical menu">
        <div class="header item">
          <i class="ui icon left laptop"></i>
          My Devices
        </div>
        <div class="item" v-for="device in store.profile.devices" v-if="hasDevices" style="padding-top: 1.1em;">
          <div>
            {{ device.name }}
            <p class="device-expires">Created: {{ device.created_at | prettyDate }}</p>
          </div>
          <button class="ui mini icon negative basic right floated button" style="margin-top: -3em;" @click="deleteDevice(device)">
            <i class="ui icon times"></i>
            Revoke
          </button>
        </div>
        <div class="item" v-if="!hasDevices" style="padding: 1.5em; text-align: center;">
          <i class="device-expires">
            You have no devices.
          </i>
        </div>
        <div class="item" v-if="creatingDevice" style="background: rgba(33, 186, 69, 0.05);">
          <div class="ui transparent input">
            <input id="newDevice"
                   type="text"
                   placeholder="Device Name"
                   v-model="newDevice.name"
                   @keyup.enter="createDevice"
                   @keyup.esc="creatingDevice = false"
                   @blur="creatingDevice = false" >
          </div>
        </div>
      </div>
      <div style="text-align: right;" v-if="canCreateDevices && !creatingDevice">
        <button class="ui button small green" @click="startCreatingDevice">
          <i class="ui plus icon"></i>
          New Device
        </button>
      </div>
    </div><!-- #Devices -->

    <!-- #RevokeDeviceModal -->
    <div id="revokeDeviceModal" class="ui basic modal">
      <div class="ui icon header">
        <i class="exclamation triangle icon red"></i>
        Revoke Device Confirmation
      </div>
      <div class="content" style="text-align: center;">
        <p>
          Revoking the device will make it no longer able to connect and will disconnect all
          of its active OpenVPN clients.
        </p>
        <p><b>Are you sure you want to revoke the device?</b></p>
      </div>
      <div class="actions" style="text-align: center">
        <button class="ui basic inverted cancel button">
          Cancel
        </button>
        <button class="ui basic red ok button">
          Revoke Device
        </button>
      </div>
    </div><!-- #RevokeDeviceModal -->

    <!-- #DownloadDeviceModal -->
    <div id="downloadModal" class="ui basic modal" style="text-align: center;">
      <div class="ui icon header">
        <i class="cloud download icon green"></i>
        Download Ready
      </div>
      <div class="content" style="text-align: center;">
        <p>Please select your operating system in order to download the proper OpenVPN client configuration.</p>
        <p><b>You will only have the next 60 seconds to download the configuration.</b></p>
      </div>
      <div class="actions" style="text-align: center">
        <button class="ui basic inverted button" @click="downloadDevice(false)">
          <i class="apple icon"></i>Apple macOS
        </button>
        <button class="ui basic inverted button" @click="downloadDevice(false)">
          <i class="windows icon"></i>Microsoft Windows
        </button>
        <button class="ui basic inverted button" @click="downloadDevice(true)">
          <i class="linux icon"></i>Linux
        </button>
      </div>
    </div><!-- #DownloadDeviceModal -->
  </div>
</template>


<script>
  import mixins from "@/utils/mixins"
  import utils from "@/utils/common"

  export default {
    name: "Profile",
    mixins: [mixins.AppComponentMixin,],
    data() {
      return {
        createdDevice: {},
        downloadModal: null,
        revokeDeviceModal: null,
        newDevice: {},
        creatingDevice: false,
      }
    },
    mounted() {
      this.revokeDeviceModal = $("#revokeDeviceModal").modal();
      this.downloadModal = $("#downloadModal").modal({
        autofocus: false,
        closable: false,
        keyboardShortcuts: false,
      });
    },
    methods: {
      // createDevice creates the new device and displays the Download modal.
      createDevice() {
        this.axios.post("/devices/", this.newDevice).then(resp => {
          this.store.profile.devices.push(resp.data);
          this.creatingDevice = false;
          this.createdDevice = resp.data;
          this.downloadModal.modal("show");
          this.newDevice = {};
        });
      },

      // deleteDevice displays the confirmation modal and deletes the given device.
      deleteDevice(device) {
        this.showModal("revokeDeviceModal", {
          onApprove: ($element) => {
            this.axios.delete(`/devices/${device.id}/`).then(resp => {
              utils.deleteFromObject(this.store.profile.devices, "id", device.id);
              this.toastr.success("The device was deleted.", "Deleted");
            });
          },
        });
      },

      // downloadDevice downloads the most recently created device.
      downloadDevice(is_linux) {
        window.location.href = `/api/devices/${this.createdDevice.id}?is_linux=${is_linux}`;
        this.createdDevice = {};
        this.hideModals();
      },

      startCreatingDevice() {
        this.newDevice = {};
        this.creatingDevice = true;

        new Promise(resolve => setTimeout(resolve, 100))
          .then(() => {
            $("#newDevice").focus();
          })
      }
    }, // #Methods
    computed: {
      /**
       * Returns whether the user can create additional devices.
       * @returns {boolean}
       */
      canCreateDevices: function() {
        return this.store.profile.devices !== undefined &&
          this.store.profile.devices.length < this.store.profile.group.max_devices;
      },

      /**
       * Returns whether the user has any devices.
       * @returns {boolean}
       */
      hasDevices: function() {
        console.log(this.store.profile);
        return this.store.profile.devices !== undefined && this.store.profile.devices.length > 0;
      },
    }
  }
</script>


<style>
  .device-expires {
    color: #999;
    font-size: 0.945em;
    padding-top: 0.33em;
  }

  div.console {
    font-family: "Consolas",sans-serif;
    font-size: 0.9em !important;
    padding: 1.5em !important;
  }
</style>
