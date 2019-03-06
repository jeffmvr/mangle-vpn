<template>
  <div class="ui row">
    <div id="pageHeader">
      <h2>New Group</h2>
    </div>

    <!-- #Form -->
    <div class="ui form">
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
              <input type="text" id="groupName" class="input" v-model="group.name">
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

          <!-- #Key Expires
          <form-table-row>
            <template slot="label">
              Device Lifetime
            </template>
            <template slot="help">
              The number of days from when a device is created until that device
              is considered expired and no longer valid and users will be required
              to create a new device configuration.
            </template>
            <template slot="input">
              <input type="text" class="input" v-model="group.key_expires">
              <p class="form-error">
                {{ errors.key_expires | error }}
              </p>
            </template>
          </form-table-row> #Key Expires -->

          <!-- #Key Expires -->
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
              <p class="form-error">
                {{ errors.max_devices | error }}
              </p>
            </template>
          </form-table-row><!-- #Key Expires -->
        </tbody>
      </table>
    </div><!-- #Form -->

    <!-- #Actions -->
    <div class="form-actions">
      <a href="/#/admin/groups" class="ui button">
        Cancel
      </a>
      <button class="ui button green" @click="createGroup">
        Create
      </button>
    </div><!-- #Actions -->
  </div>
</template>


<script>
  import mixins from '@/utils/mixins'
  import FormTableRow from '@/components/common/FormTableRow'

  export default {
    name: "GroupCreate",
    mixins: [mixins.AppComponentMixin, ],
    components: {FormTableRow, },
    data() {
      return {
        errors: {},
        group: {
          key_expires: 365,
          max_devices: 2,
        },
      }
    },
    mounted() {
      document.getElementById("groupName").focus();
    },
    methods: {
      // createGroup creates the new group.
      createGroup() {
        this.axios.post("/admin/groups/", this.group).then(resp => {
          this.toastr.success("The group was created.", "Created");
          this.$router.push("/#/admin/groups");
        }).catch(err => {
          this.errors = err.response.data;
        });
      },
    }, // #Methods
  }
</script>
