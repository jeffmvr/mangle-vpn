<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="seven wide column">
        <h2>{{ group.name }}</h2>
      </div>
      <div class="nine wide column page-actions">
        <router-link :to="{ name: 'UserCreate', params: { groupID: group.id } }" class="ui tiny basic green button">
          Add Users
        </router-link>
        <button class="ui tiny negative button" @click="showModal('deleteGroupModal')">
          Delete Group
        </button>
      </div>
    </div><!-- #Header -->

    <!-- #Tabs -->
    <div class="ui pointing secondary menu">
      <a class="active item" data-tab="details">
        <i class="cog icon left"></i> Details
      </a>
      <a class="item" data-tab="firewall">
        <i class="fire icon left"></i> Firewall
      </a>
      <a class="item" data-tab="users">
        <i class="users icon left"></i> Users
      </a>
    </div><!-- #Tabs -->

    <!-- #DetailsTab -->
    <div class="ui active tab form" data-tab="details">
      <table class="ui very basic table">
        <tbody>
          <!-- #Name -->
          <form-table-row>
            <template slot="label">
              Group Name
            </template>
            <template slot="help">
              A unique name given to the group which describes its purpose and
              the users and devices that are members of it.
            </template>
            <template slot="input">
              <input type="text" id="name" class="input" v-model="group.name">
              <p class="form-error">
                {{ errors.name | error }}
              </p>
            </template>
          </form-table-row><!-- #Name -->

          <!-- #Description -->
          <form-table-row>
            <template slot="label">
              Description
            </template>
            <template slot="help">
              Describes the purpose of the group or any special notes that might be
              of interest to adminisrators.
            </template>
            <template slot="input">
              <textarea class="textarea" rows="5" v-model="group.description"></textarea>
            </template>
          </form-table-row><!-- #Description -->

          <!-- #Status -->
          <form-table-row>
            <template slot="label">
              Status
            </template>
            <template slot="help">
              The current status of the group, either enabled or disabled. When
              disabled, any user who is part of the group will not be able to connect
              to the VPN and any clients currently connected will be disconnected
              automatically.
            </template>
            <template slot="input">
              <select id="statusDropdown" class="ui dropdown" v-model="group.is_enabled">
                <option :value="true">Enabled</option>
                <option :value="false">Disabled</option>
              </select>
            </template>
          </form-table-row><!-- #Status -->

          <!-- #MaxDevices -->
          <form-table-row>
            <template slot="label">
              Maximum Devices
            </template>
            <template slot="help">
              The maximum number of non-revoked devices each user in the group
              is allowed to have at any one time. When this number is reached,
              the user will not be allowed to create any additional devices.
            </template>
            <template slot="input">
              <input type="text" class="input" v-model="group.max_devices">
              <p class="form-errors">
                {{ errors.max_devices | error }}
              </p>
            </template>
          </form-table-row><!-- #MaxDevices -->
        </tbody>
      </table>

      <!-- #Actions -->
      <div class="form-actions">
        <a href="/#/admin/groups" class="ui button">
          Cancel
        </a>
        <button class="ui button green" @click="updateGroup()">
          Update
        </button>
      </div><!-- #Actions -->
    </div><!-- #DetailsTab -->

    <!-- #FirewallRulesTab -->
    <div class="ui tab" data-tab="firewall">
      <table class="ui selectable table">
        <thead>
          <tr>
            <th style="width: 5%;"></th>
            <th style="width: 25%;">Destination</th>
            <th style="width: 20%; text-align: center;">Protocol</th>
            <th style="width: 35%;">Ports</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rule in firewall" class="rule">
            <td>
              <span v-if="rule.action === 'ACCEPT'">
                <i class="ui green check circle icon"></i>
              </span>
              <span v-if="rule.action === 'DROP'">
                <i class="ui red ban icon"></i>
              </span>
            </td>
            <td>{{ rule.destination | firewallRuleAny }}</td>
            <td style="text-align: center;">{{ rule.protocol | firewallRuleAny }}</td>
            <td>{{ rule.port | firewallRuleAny }}</td>
            <td style="text-align: right;">
              <button class="ui mini basic icon red button" @click="deleteFirewallRule(rule)">
                <i class="ui times icon"></i>
                Delete
              </button>
            </td>
          </tr>
          <tr class="rule disabled">
            <td><i class="ui red ban icon"></i></td>
            <td>Anywhere</td>
            <td style="text-align: center;">All</td>
            <td>All</td>
            <td></td>
          </tr>
        </tbody>
      </table>

      <div class="page-buttons">
        <button class="ui tiny right icon green button" @click="showModal('firewallRuleModal')">
          <i class="plus icon"></i> Add Rule
        </button>
      </div>
    </div><!-- #FirewallRulesTab -->

    <!-- #UsersTab -->
    <div class="ui tab" data-tab="users">
      <table class="ui selectable table" v-if="users.length > 0">
        <thead>
          <tr>
            <th style="width: 66%;">E-mail Address</th>
            <th>Last Login</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users">
            <td>
              <router-link :to="{ name: 'UserDetail', params: { id: user.id } }">
                {{ user.email }}
              </router-link>
            </td>
            <td>
              {{ user.last_login | prettyDateTime }}
            </td>
          </tr>
        </tbody>
      </table>

      <div class="empty" v-else>
        <i>There are no users in the group.</i>
      </div>
    </div><!-- #UsersTab -->

    <!-- #FirewallRuleModal -->
    <div id="firewallRuleModal" class="ui tiny modal form">
      <div class="header">
        New Firewall Rule
      </div>
      <div class="content">
        <table class="ui very basic table">
          <!-- #Destination -->
          <form-table-row>
            <template slot="label">
              Destination
            </template>
            <template slot="help">
              An IPv4 CIDR or IP address of the destination host(s).
            </template>
            <template slot="input">
              <input type="text" class="input" placeholder="Destination" v-model="firewallRule.destination">
              <p class="form-error">
                {{ errors.destination | error }}
              </p>
            </template>
          </form-table-row><!-- #Destination -->

          <!-- #Ports -->
          <form-table-row>
            <template slot="label">
              Ports
            </template>
            <template slot="help">
              A port number, group of port numbers separated by commas, or range
              of port numbers separated by a colon.
            </template>
            <template slot="input">
              <input type="text" class="ui input" placeholder="Any" v-model="firewallRule.port">
              <p class="form-error">
                {{ errors.port | error }}
              </p>
            </template>
          </form-table-row><!-- #Ports -->

          <!-- #Protocol -->
          <form-table-row>
            <template slot="label">
              Protocol
            </template>
            <template slot="help">
              The networking protocol that will be permitted.
            </template>
            <template slot="input">
              <select id="protocolDropdown" class="ui dropdown" v-model="firewallRule.protocol">
                <option value="all">All</option>
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
              <p class="form-error">
                {{ errors.protocol | error }}
              </p>
            </template>
          </form-table-row><!-- #Protocol -->

          <!-- #Action -->
          <form-table-row>
            <template slot="label">
              Action
            </template>
            <template slot="help">
              The action to take for traffic that matches the firewall rule. Must be either
              ACCEPT or DROP.
            </template>
            <template slot="input">
              <select id="actionDropdown" class="ui dropdown" v-model="firewallRule.action">
                <option value="accept">ALLOW</option>
                <option value="drop">DENY</option>
              </select>
              <p class="form-error">
                {{ errors.action | error }}
              </p>
            </template>
          </form-table-row><!-- #Status -->
        </table>
      </div>
      <div class="actions">
        <button class="ui button" @click="hideModals()">
          Cancel
        </button>
        <button class="ui green button" @click="createFirewallRule()">
          Add Rule
        </button>
      </div>
    </div><!-- #FirewallRuleModal -->

    <!-- #DeleteGroupModal -->
    <div id="deleteGroupModal" class="ui basic modal">
      <div class="ui icon header">
        <i class="exclamation triangle icon red"></i>
        Delete Group Confirmation
      </div>
      <div class="content" style="text-align: center;">
        <p>
          Deleting the Group will also delete all of its users, devices, and will
          disconnect any active OpenVPN clients.
        </p>
        <p><b>Are you sure you want to delete the group?</b></p>
      </div>
      <div class="actions" style="text-align: center">
        <button class="ui basic inverted button" @click="hideModals()">
          Cancel
        </button>
        <button class="ui basic button red" @click="deleteGroup()">
          Delete Group
        </button>
      </div>
    </div><!-- #DeleteGroupModal -->
  </div>
</template>


<script>
  import mixins from '@/utils/mixins'
  import FormTableRow from '@/components/common/FormTableRow'

  export default {
    name: "GroupDetail",
    components: {FormTableRow, },
    mixins: [mixins.AppComponentMixin,],
    data() {
      return {
        deleteGroupModal: null,
        firewallRuleModal: null,
        errors: {},
        group: {},
        firewall: [],
        firewallRule: {},
        users: [],
      }
    },
    mounted() {
      this.getGroup();

      // Modals
      this.deleteGroupModal = $("#deleteGroupModal").modal({
        autofocus: false,
      });

      this.firewallRuleModal = $("#firewallRuleModal").modal({
          onHide: this.onHideFirewallRuleModal,
          onShow: this.onShowFirewallRuleModal,
      });

      // Reloads the data for the active tab
      $(".secondary.menu .item").tab({
        onLoad: (tabPath, parameterArray, historyEvent) => {
          this.errors = [];
          switch (tabPath) {
            case "details":
              this.getGroup();
              break;
            case "firewall":
              this.getFirewallRules();
              break;
            case "users":
              this.getUsers();
              break;
          }
        },
      });
    }, // #Mounted
    beforeDestroy() {
      // Remove the modal to reset inputs properly
      // This is a Semantic + Vue issue (see: https://github.com/Semantic-Org/Semantic-UI/issues/3200)
      this.firewallRuleModal.remove();
    },
    methods: {
      /**
       * Retrieves the group's details.
       * @returns {null}
       */
      getGroup() {
        this.axios.get(`/admin/groups/${this.$route.params.id}/`)
          .then(resp => {
            this.group = resp.data;
            $("#statusDropdown").dropdown("set selected", this.group.is_enabled);
          });
      },

      /**
       * Updates the group's details.
       * @returns {null}
       */
      updateGroup() {
        this.axios.put(`/admin/groups/${this.$route.params.id}/`, this.group)
          .then(resp => {
            this.toastr.success("The group was updated.", "Updated");
          })
          .catch(err => {
            this.errors = err.response.data;
          });
      },

      // deleteGroup deletes the group and redirects back to the main groups page.
      deleteGroup() {
        this.axios.delete(`/admin/groups/${this.$route.params.id}/`)
          .then(resp => {
            this.hideModals();
            this.$router.push("/admin/groups");
            this.toastr.success("The group was deleted.", "Deleted");
          });
      },

      // getFirewallRules retrieves all of the Group's firewall rules.
      getFirewallRules() {
        this.axios.get(`/admin/groups/${this.$route.params.id}/firewall/`)
          .then(resp => {
            this.firewall = resp.data;
          });
      },

      // createFirewallRule creates a new firewall rule.
      createFirewallRule() {
        this.firewallRule.group_id = this.group.id;
        this.axios.post(`/admin/firewall/`, this.firewallRule)
          .then(resp => {
            this.toastr.success("The firewall rule was added", "Created");
            this.getFirewallRules();
            this.hideModals();
            this.firewallRule = {};
            this.errors = [];
          })
          .catch(err => {
            this.errors = err.response.data;
          })
      },

      // deleteFirewallRule deletes the given firewall rule.
      deleteFirewallRule(rule) {
        this.axios.delete(`/admin/firewall/${rule.id}/`)
          .then(resp => {
            this.getFirewallRules();
            this.toastr.success("The firewall rule was deleted", "Deleted")
          });
      },

      getUsers() {
        this.axios.get(`/admin/groups/${this.group.id}/users`)
          .then(resp => {
            this.users = resp.data;
          });
      },

      // toggleFirewallRule toggles the ``is_enabled`` value of the given rule.
      toggleFirewallRule(rule) {
        rule.is_enabled = !rule.is_enabled;
        this.axios.put(`/admin/firewall/${rule.id}/`, rule)
          .then(resp => {
            this.toastr.success("The firewall rule was updated", "Updated");
          });
      },

      // onShowFirewallRuleModal resets the new firewall rule when the modal is shown.
      onShowFirewallRuleModal() {
        this.firewallRule = {};
        $("#actionDropdown").dropdown("set selected", "accept");
        $("#protocolDropdown").dropdown("set selected", "all");
      },

      // onHideFirewallRuleModal resets the new firewall rule when the modal is hidden.
      onHideFirewallRuleModal(el) {
        this.firewallRule = {};
      }
    },
  }
</script>

<style>
  tr.rule.disabled > td:last-child {
    pointer-events: auto !important;
  }

  tr.rule.disabled > td:last-child > button {
    margin-left: 1em;
  }
</style>
