<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="four wide column">
        <h2>{{ user.email }}</h2>
      </div>
      <div class="twelve wide column page-actions">
        <button class="ui tiny button" @click="showModal('resetTwoFactorModal')">
          Reset 2FA
        </button>
        <button class="ui tiny negative button" @click="showModal('deleteUserModal')">
          Delete User
        </button>
      </div>
    </div><!-- #Header -->

    <!-- #Tabs -->
    <div class="ui pointing secondary menu">
      <a class="active item" data-tab="details">
        <i class="cog icon let"></i> Details
      </a>
      <a class="item" data-tab="devices">
        <i class="laptop icon left"></i> Devices
      </a>
    </div><!-- #Tabs -->

    <!-- #UserDetails -->
    <div class="ui active tab form" data-tab="details">
      <table class="ui very basic table">
        <tbody>
          <!-- #Email -->
          <form-table-row>
            <template slot="label">
              E-mail Address
            </template>
            <template slot="help">
              The user's e-mail address and username when logging into the web
              application and connecting to the OpenVPN server.
            </template>
            <template slot="input">
              <input type="text" class="input" v-model="user.email" disabled="disabled">
              <p class="form-error">
                {{ errors.email | error }}
              </p>
            </template>
          </form-table-row><!-- #Name -->

          <!-- #FullName -->
          <form-table-row>
            <template slot="label">
              Full Name
            </template>
            <template slot="help">
              The user's full name as set in their OAuth2 account profile and
              will remain blank until the user logs in for the first time.
            </template>
            <template slot="input">
              <input type="text" class="input" v-model="user.name" disabled="disabled">
              <p class="form-error">
                {{ errors.name | error }}
              </p>
            </template>
          </form-table-row><!-- #FullName -->

          <!-- #Role -->
          <form-table-row>
            <template slot="label">
              Role
            </template>
            <template slot="help">
              The user's role within the application, which determines their
              application permissions and capabilities.
            </template>
            <template slot="input">
              <select id="userIsAdmin" class="ui dropdown" v-model="user.is_admin">
                <option :value="true" :selected="user.is_admin === true">Administrator</option>
                <option :value="false" :selected="user.is_admin === false">Regular User</option>
              </select>
              <p class="form-error">
                {{ errors.is_admin | error }}
              </p>
            </template>
          </form-table-row><!-- #Role -->

          <!-- #Status -->
          <form-table-row>
            <template slot="label">
              Status
            </template>
            <template slot="help">
              Whether the user is able to log in to the application and OpenVPN
              server. When set to 'Disabled', the user will have no access to
              either application.
            </template>
            <template slot="input">
              <select id="userIsEnabled" class="ui dropdown" v-model="user.is_enabled">
                <option :value="true">Enabled</option>
                <option :value="false">Disabled</option>
              </select>
              <p class="form-error">
                {{ errors.is_enabled | error }}
              </p>
            </template>
          </form-table-row><!-- #Status -->

          <!-- #Group -->
          <form-table-row>
            <template slot="label">
              Group
            </template>
            <template slot="help">
              The group to which each user will be assigned, and used to enforce
              access control and other administration settings.
            </template>
            <template slot="input">
              <select id="userGroup" class="ui dropdown" v-model="user.group_id">
                <option v-for="group in groups" :value="group.id">
                  {{ group.name }}
                </option>
              </select>
              <p class="form-error">
                {{ errors.group_id | error }}
              </p>
            </template>
          </form-table-row><!-- #Group -->

          <!-- #TwoFactorAuthentication -->
          <form-table-row>
            <template slot="label">
              Two-Factor Authentication
            </template>
            <template slot="help">
              Determines whether the is required to setup and use two-factor authentication
              when logging into the web application and connecting to the OpenVPN server. This
              will override any group settings.
            </template>
            <template slot="input">
              <select id="mfaEnforcedDropdown" class="ui dropdown" v-model="user.mfa_enforced">
                <option value="inherit">Inherit From Group</option>
                <option :value="true">Required</option>
                <option :value="false">Not Required</option>
              </select>
            </template>
          </form-table-row><!-- #TwoFactorAuthentication -->
        </tbody>
      </table>

      <!-- #Actions -->
      <div class="form-actions">
        <a href="/#/admin/users" class="ui button">
          Cancel
        </a>
        <button class="ui button green" @click="updateUser()">
          Update
        </button>
      </div><!-- #Actions -->
    </div><!-- #UserDetails -->

    <!-- Devices -->
    <div class="ui tab" data-tab="devices">
      <div v-if="devices.length > 0">
        <table class="ui selectable table">
          <thead>
            <tr>
              <th style="width: 30%;">Name</th>
              <th style="width: 30%;">Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="device in devices">
              <td>{{ device.name }}</td>
              <td>{{ device.created_at | prettyDateTime }}</td>
              <td style="text-align: right;">
                <button class="ui mini negative basic button" @click="deleteDevice(device)">
                  Revoke
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="empty" v-else>
        <i>The user has no devices.</i>
      </div>
    </div> <!-- # Devices -->

    <!-- #DeleteUserModal -->
    <div id="deleteUserModal" class="ui basic modal">
      <div class="ui icon header">
        <i class="exclamation triangle icon red"></i>
        Delete User Confirmation
      </div>
      <div class="content" style="text-align: center;">
        <p>
          Deleting the user will also delete all of their devices and will
          disconnect any active VPN connections.
        </p>
        <p><b>Are you sure you want to delete the user?</b></p>
      </div>
      <div class="actions" style="text-align: center">
        <button class="ui basic inverted button" @click="hideModals()">
          Cancel
        </button>
        <button class="ui basic button red" @click="deleteUser()">
          Delete User
        </button>
      </div>
    </div><!-- #DeleteUserModal -->

    <!-- #ResetTwoFactorModak -->
    <div id="resetTwoFactorModal" class="ui basic modal">
      <div class="ui icon header">
        <i class="exclamation triangle icon red"></i>
        Confirm Two-Factor Authentication Reset
      </div>
      <div class="content" style="text-align: center;">
        <p>
          Resetting the user's two-factor authentication will force them to
          re-add it to their mobile app on their next login and their old
          two-factor authentication will no longer work.
        </p>
        <p><b>Are you sure you want to reset the user's two-factor authentication?</b></p>
      </div>
      <div class="actions" style="text-align: center">
        <button class="ui basic inverted button" @click="hideModals()">
          Cancel
        </button>
        <button class="ui basic button red" @click="resetTwoFactor()">
          Reset Two-Factor
        </button>
      </div>
    </div><!-- #ResetTwoFactorModak -->

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
  </div>
</template>


<script>
  import BaseMixin from '@/components/mixins/BaseMixin'
  import FormTableRow from "@/components/common/FormTableRow"

  export default {
    name: "UserDetail",
    mixins: [
      BaseMixin,
    ],
    components: {
      FormTableRow,
    },
    data() {
      return {
        deleteUserModal: null,
        resetPasswordModal: null,
        resetTwoFactorModal: null,
        revokeDeviceModal: null,
        devices: [],
        errors: {},
        user: {},
        groups: [],
      }
    },
    mounted() {
      this.getUser();

      // Delete user confirmation modal
      this.deleteUserModal = $("#deleteUserModal").modal({
        autofocus: false,
      });

      // Two-factor authentication reset confirmation modal
      this.resetTwoFactorModal = $("#resetTwoFactorModal").modal({
        autofocus: false
      });

      // Revoked device confirmation modal
      this.revokeDeviceModal = $("#revokeDeviceModal");

      // Reloads the data for the active tab
      $(".secondary.menu .item").tab({
        onLoad: (tabPath, parameterArray, historyEvent) => {
          this.errors = [];
          switch (tabPath) {
            case "details":
              this.getUser();
              break;
            case "devices":
              this.getDevices();
              break;
          }
        },
      });
    },
    methods: {
      /**
       * Retrieves the user details and list of groups.
       * @returns {null}
       */
      getUser() {
        this.axios.get(`/admin/users/${this.$route.params.id}/`)
          .then(resp => {
            this.user = resp.data;
            $("#userIsAdmin").dropdown("set selected", this.user.is_admin);
            $("#userIsEnabled").dropdown("set selected", this.user.is_enabled);

            if (this.user.mfa_enforced === null) {
              this.user.mfa_enforced = "inherit";
            }

            $("#mfaEnforcedDropdown").dropdown("set selected", this.user.mfa_enforced);
            // Retrieve the list of application groups to populate the dropdown
            // and set the user's group. This is done after the user is retrieved
            // to ensure the dropdown is proerly populated
            this.axios.get("/admin/groups/all/")
              .then(resp => {
                this.groups = resp.data;
                $("#userGroup").dropdown("set selected", this.user.group.id);
              });
          });
      },

      /**
       * Updates the user details.
       * @returns {null}
       */
      updateUser() {
        if (this.user.mfa_enforced === "inherit") {
          this.user.mfa_enforced = null;
        }

        this.axios.put(`/admin/users/${this.$route.params.id}/`, this.user)
          .then(resp => {
            this.user = resp.data;
            this.toastr.success("The user was updated.", "Updated")
          })
          .catch(err => {
            this.errors = err.response.data;
          });
      },

      /**
       * Resets the user two-factor authentication secret and status.
       * @returns {null}
       */
      resetTwoFactor() {
        this.axios.put(`/admin/users/${this.$route.params.id}/mfa/`)
          .then(resp => {
            this.hideModals();
            this.toastr.success("The user's two-factor auth was reset.", "Updated");
          });
      },

      /**
       * Deletes the user.
       * @returns {null}
       */
      deleteUser() {
        this.axios.delete(`/admin/users/${this.$route.params.id}/`)
          .then(resp => {
            this.hideModals();
            this.$router.push("/admin/users");
            this.toastr.success("The user was deleted.", "Deleted");
          });
      },

      /**
       * Retrieves all of the user devices.
       * @returns {null}
       */
      getDevices() {
        this.axios.get(`/admin/users/${this.$route.params.id}/devices/`)
          .then(resp => {
            this.devices = resp.data;
          });
      },

      /**
       * Deletes the given user device.
       * @param {object} device
       * @returns {null}
       */
      deleteDevice(device) {
        this.showModal("revokeDeviceModal", {
          autofocus: false,
          onApprove: ($element) => {
            this.axios.delete(`/admin/devices/${device.id}/`)
              .then(resp => {
                this.toastr.success("The device was deleted.", "Deleted");
                this.getDevices();
              });
          },
        });
      },
    }, // #Methods
  }
</script>
