<template>
  <div class="ui row">
    <!-- #Instructions -->
    <div class="ten wide column">
      <h1>Getting Started</h1>
      <p>
        Generate and download an OpenVPN configuration file for each of your devices by clicking the
        <a href="#" @click="showModal('createDeviceModal')">New Device</a> button on the right and follow the instructions
        below for your device's operating system.
      </p>

      <!-- #Authentication -->
      <h3 style="margin-top: 2.5em;">
        <i class="lock icon"></i>&nbsp;OpenVPN Authentication
      </h3>

      <div class="ui divider"></div>
      <p>
        In order to connect to the OpenVPN server with your devices, you are required to authenticate before each session
        using the following credentials:
      </p>
      <div class="ui message" style="padding: 2em 0;">
        <table style="margin: 0 auto;">
          <tr>
            <td style="font-weight: bold; width: 25%;">Username:</td>
            <td>{{ store.profile.email }}</td>
          </tr>
          <tr>
            <td style="font-weight: bold; width: 25%;">Password:</td>
            <td style="font-style: italic">
              <span v-if="store.profile.mfa_required">the two-factor auth code from your mobile app</span>
              <span v-else>Not Required</span>
            </td>
          </tr>
        </table>
      </div><!-- #Authentication -->

      <!-- #ClientSetup -->
      <h3 style="margin-top: 2.5em;">
        <i class="window restore icon"></i>&nbsp;OpenVPN Client Setup
      </h3>

      <div class="ui divider"></div>
      <p>
        Follow the instructions for your device's operating system in order to setup your OpenVPN client and connect to
        the OpenVPN server.
      </p>
      <p>
        In addition to the desktop clients, the OpenVPN Connect mobile application is available from both the
        <a href="https://play.google.com/store/apps/details?id=net.openvpn.openvpn">Android Play Store</a> and
        <a href="https://itunes.apple.com/us/app/openvpn-connect/id590379981?mt=8">Apple App Store</a>.
      </p>

      <!-- #Tabs -->
      <div class="ui pointing secondary menu" style="margin-top: 2em;">
        <a class="active item" data-tab="macos">
          <i class="apple icon left"></i> Apple macOS
        </a>
        <a class="item" data-tab="windows">
          <i class="windows icon left"></i> Microsoft Windows
        </a>
        <a class="item" data-tab="linux">
          <i class="linux icon left"></i> Ubuntu 16.04+
        </a>
      </div><!-- #Tabs -->

      <!-- #macOS -->
      <div class="ui active tab" data-tab="macos">
        <p>
          The following instructions describe how to install and configure the OpenVPN client when using an Apple macOS
          device, such as a Macbook or Mac Pro.
        </p>
        <ol>
          <li>
            Download and install the <a href="https://tunnelblick.net/release/Tunnelblick_3.7.6a_build_5080.dmg">
            Tunnelblick</a> client
          </li>
          <li>Download and double click a device configuration file (.ovpn) to import it</li>
          <li>Click the Tunnelblick application icon on the macOS status bar and click Connect</li>
          <li>Enter your username (e-mail address) and password (the two-factor authentication code from your mobile app)</li>
        </ol><!-- #macOS -->
      </div>

      <!-- #Windows -->
      <div class="ui tab" data-tab="windows">
        <p>
          The following instructions describe how to install and configure the OpenVPN client when using a Microsoft
          Windows device, such as a laptop or desktop.
        </p>

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
          <li>Enter your username (e-mail address) and password (the two-factor authentication code from your mobile app)</li>
        </ol>
      </div><!-- #Windows -->

      <!-- #Linux -->
      <div class="ui tab" data-tab="linux">
        <p>
          Download and run the <a href="/scripts/openvpn-client-installer.sh">OpenVPN client installation script</a> which
          will handle installation of OpenVPN and additional required packages.
        </p>
        <div class="ui message console">
          $ chmod a+x openvpn-client-installer.sh <br>
          $ sudo ./openvpn-client-installer.sh
        </div><!-- #Linux -->
        <p>
          Once OpenVPN has been installed, you can connect to the OpenVPN server using the following command. Be sure to
          enter your username (e-mail address) and password (the two-factor authentication code from your mobile app):
        </p>
        <div class="ui message console">
          $ sudo openvpn --config /path/to/config.ovpn
        </div>
      </div><!-- #Linux -->
    </div><!-- #ClientSetup -->

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
            You have no devices :(
          </i>
        </div>
      </div>
      <div style="text-align: right;" v-if="canCreateDevices">
        <button class="ui button small green" @click="showModal('createDeviceModal')">
          <i class="ui plus icon"></i>
          New Device
        </button>
      </div>

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

    <!-- #DeviceModal -->
    <div id="createDeviceModal" class="ui small modal">
      <div class="header">
        <i class="ui icon left laptop"></i>
        New Device Configuration
      </div>

      <div class="content" style="padding: 2em 3em 1em 3em;">
        <p>
          You must create and install an OpenVPN configuration file for each device you wish to
          connect with to the OpenVPN server.
        </p>

        <h4>OpenVPN Credentials</h4>
        <p>
          In order to connect to the OpenVPN server with your devices, you are required to authenticate before each session
          using the following credentials:
        </p>

        <!-- #Credentials -->
        <div class="ui message" style="padding: 2em 0;">
          <table style="margin: 0 auto;">
            <tr>
              <td style="font-weight: bold; width: 25%;">Username:</td>
              <td>{{ store.profile.email }}</td>
            </tr>
            <tr>
              <td style="font-weight: bold; width: 25%;">Password:</td>
              <td style="font-style: italic">the two-factor auth code from your mobile app</td>
            </tr>
          </table>
        </div><!-- #Credentials -->

        <div class="ui form" style="margin: 2em 0 1em 0;">
          <table class="ui very basic table">
            <tr>
              <td>
                <label style="font-weight: bold;">Device Name</label>
              </td>
              <td>
                <input type="text" v-model="newDevice.name">
                <p class="form-error">
                  {{ errors.name | error }}
                </p>
              </td>
            </tr>

            <!-- #OperatingSystem -->
            <tr>
              <td>
                <label style="font-weight: bold;">Operating System</label>
              </td>
              <td>
                <select id="operatingSystemDropdown" class="ui dropdown" v-model="newDevice.os">
                  <option value="">Select Operating System</option>
                  <option value="windows">Microsoft Windows</option>
                  <option value="macos">Apple macOS</option>
                  <option value="linux">Linux</option>
                </select>
                <p class="form-error">
                  {{ errors.os | error }}
                </p>
              </td>
            </tr><!-- #OperatingSystem -->
          </table>
        </div>
      </div>

      <div class="actions" style="text-align: center">
        <button class="ui cancel button">
          Cancel
        </button>
        <button id="downloadConfigButton" class="ui green button" @click="createDevice">
          Download Configuration
        </button>
      </div>
    </div>
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
        createDeviceModal: null,
        createdDevice: {},
        downloadModal: null,
        errors: {},
        revokeDeviceModal: null,
        newDevice: {},
        creatingDevice: false,
      }
    },
    mounted() {
      $(".secondary.menu .item").tab();

      this.createDeviceModal = $("#createDeviceModal").modal({
        onShow: this.onCreateDeviceModalShow,
        onHide: this.onCreateDeviceModalHide,
      });

      this.revokeDeviceModal = $("#revokeDeviceModal").modal();
    },
    methods: {
      // createDevice creates the new device and displays the Download modal.
      createDevice() {
        $("#downloadConfigButton").addClass("loading disabled");

        this.axios.post("/devices/", this.newDevice)
          .then(resp => {
            this.store.profile.devices.push(resp.data);
            this.creatingDevice = false;
            this.createdDevice = resp.data;
            this.downloadDevice(this.newDevice.os);
            this.newDevice = {};
          })
          .catch(err => {
            $("#downloadConfigButton").removeClass("loading disabled");
            this.errors = err.response.data;
          });
      },

      // deleteDevice displays the confirmation modal and deletes the given device.
      deleteDevice(device) {
        this.showModal("revokeDeviceModal", {
          onApprove: ($element) => {
            this.axios.delete(`/devices/${device.id}/`).then(resp => {
              utils.deleteFromObject(this.store.profile.devices, "id", device.id);
              this.toastr.success("The device was deleted.", "Device deleted");
            });
          },
        });
      },

      // downloadDevice downloads the most recently created device.
      downloadDevice(os) {
        window.location.href = `/api/devices/${this.createdDevice.id}?os=${os}`;
        this.createdDevice = {};
        this.hideModals();
        this.toastr.success("Your device was downloaded.", "Device Created");
      },

      onCreateDeviceModalShow(el) {
        $("#downloadConfigButton").removeClass("loading disabled");
        $("#operatingSystemDropdown").dropdown("restore defaults");
      },

      onCreateDeviceModalHide(el) {
        this.errors = {};
        this.newDevice = {};
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
