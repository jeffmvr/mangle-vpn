<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="four wide column" style="margin-top: 0.3em;">
        <h2>Invite Users</h2>
      </div>
      <div class="twelve wide column page-actions">

      </div>
    </div><!-- #Header -->

    <!-- #Form -->
    <div class="ui form">
      <table class="ui very basic table">
        <tbody>
        <!-- #Emails -->
        <form-table-row>
          <template slot="label">
            E-mail Addresses
          </template>
          <template slot="help">
            A list of the user email addresses, one per line, for users
            who will be granted access to the application and OpenVPN server.
          </template>
          <template slot="input">
            <textarea id="emails" rows="6" v-model="user.email"></textarea>
            <p class="form-error">
              {{ errors.email | error }}
            </p>
          </template>
        </form-table-row><!-- #Emails -->

        <!-- #Group -->
        <form-table-row>
          <template slot="label">
            Group
          </template>
          <template slot="help">
            The group to which each user will be assigned, used to enforce
            access control and other administrative settings.
          </template>
          <template slot="input">
            <select id="groupID" class="ui selection dropdown" v-model="user.group_id">
              <option disabled selected="selected" value="">
                Select Group
              </option>
              <option v-for="group in groups" :value="group.id">
                {{ group.name }}
              </option>
            </select>
            <p class="form-error">
              {{ errors.group_id | error }}
            </p>
          </template>
        </form-table-row><!-- #Group -->

        <!-- #Notification -->
        <form-table-row>
          <template slot="label">
            Send E-mail
          </template>
          <template slot="help">
            Whether to send the user(s) a notification e-mail with instructions
            on how to setup their account.
          </template>
          <template slot="input">
            <select id="notify" class="ui dropdown" v-model="user.notify">
              <option :value="true">
                Yes
              </option>
              <option :value="false">
                No
              </option>
            </select>
          </template>
        </form-table-row><!-- #Notification -->
        </tbody>
      </table>
    </div><!-- #Form -->

    <!-- #Actions -->
    <div class="form-actions">
      <a href="/#/admin/users" class="ui button">
        Cancel
      </a>
      <button class="ui button green" @click="create">
        Invite
      </button>
    </div><!-- #Actions -->

    <!-- #UsersCreatedModal -->
    <div id="usersCreatedModal" class="ui small modal">
      <i class="close icon"></i>
      <div class="header">
        Users Created
      </div>

      <div class="content">
        <p>
          The following users have been created. If you have setup application e-mails, then each
          user will receive an e-mail with instructions containing their password which must be
          changed upon logging in.
        </p>

        <table class="ui basic table">
          <thead>
          <tr>
            <th>E-mail Address</th>
            <th>Password</th>
          </tr>
          </thead>
          <tbody>
          <tr v-for="u in users">
            <td>{{ u.email }}</td>
            <td>{{ u.password }}</td>
          </tr>
          </tbody>
        </table>
      </div>

      <div class="actions" style="text-align: center;">
        <button class="ui primary button" @click="hideModals">
          Close
        </button>
      </div>
    </div><!-- #UsersCreatedModal -->
  </div>
</template>


<script>
  import BaseMixin from "@/components/mixins/BaseMixin"
  import FormTableRow from "@/components/common/FormTableRow"

  export default {
    name: "UserCreate",
    mixins: [
      BaseMixin,
    ],
    components: {
      FormTableRow,
    },
    data() {
      return {
        usersCreatedModal: null,
        errors: {},
        groups: [],
        user: {},
        users: [],
      }
    },
    mounted() {
      document.getElementById("emails").focus();

      this.usersCreatedModal = $("#usersCreatedModal").modal();

      $("#notify").dropdown("set selected", "true");
      $("#groupID").dropdown();

      // Retrieve a list of all Groups so the dropdown can be populated
      this.axios.get("/admin/groups/all")
        .then(resp => {
          this.groups = resp.data;

          $("#groupID").dropdown("refresh")

          // if the groupID is set in the route params then set it as the selected group
          // this will be set when adding users from a Group page
          // this is also wrapped in a setTimeout() due to Semantic UI dropdown not updating
          // properly after retrieving the data
          if (this.$route.params.groupID !== undefined) {
            setTimeout(() => {
              $("#groupID").dropdown("set selected", this.$route.params.groupID);
            }, 1);
          }
        });
    },

    methods: {
      /**
       * Creates the users.
       * @returns {null}
       */
      create() {
        this.axios.post("/admin/users/", this.user)
          .then(resp => {
            this.users = resp.data;
            this.showModal("usersCreatedModal", {
              onHide: () => {
                this.$router.push("/admin/users")
              },
            });
          })
          .catch(err => {
            this.errors = err.response.data;
          });
      }
    }
  }
</script>
