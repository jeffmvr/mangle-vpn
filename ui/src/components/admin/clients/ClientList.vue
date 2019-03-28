<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="seven wide column">
        <h2>VPN Clients</h2>
      </div>
      <div class="nine wide column page-actions">
        <div class="ui icon mini input search">
          <i class="search icon"></i>
          <input class="input" type="text" placeholder="Search Clients..." v-model="search" @keyup="performSearchOnType(getClients)">
        </div>
      </div>
    </div><!-- #Header -->

    <div v-if="clients.length > 0">
      <table class="ui selectable table">
        <thead>
          <tr>
            <th style="width: 30%;">User</th>
            <th style="width: 20%;">IP Address</th>
            <th style="width: 25%;">Device</th>
            <th style="width: 15%;">Duration</th>
            <th></th>
          </tr>
        </thead>

        <tbody>
          <tr v-for="client in clients">
            <td>
              <a :href="`/#/admin/users/${client.device.user.id}`">
                {{ client.device.user.email }}
              </a>
            </td>
            <td>{{ client.virtual_ip }}</td>
            <td>{{ client.device.name }}</td>
            <td>{{ client.duration | secsToHoursMins }}</td>
            <td>
              <button class="ui mini basic negative button" @click="deleteClient(client)">
                Disconnect
              </button>
            </td>
          </tr>
        </tbody>
      </table>

      <div class="page-buttons">
        <span>{{ total }} Total</span>
        <button :class="['ui tiny button', { 'disabled': !hasPrevPage }]" @click="prevPage(getEvents)">
          <i class="angle left icon"></i>
          Prev
        </button>
        <button :class="['ui tiny right button', { 'disabled': !hasNextPage }]" @click="nextPage(getEvents)">
          Next
          <i class="angle right icon"></i>
        </button>
      </div>
    </div>

    <div class="empty" style="padding: 2.5em 0;" v-else>
      <i style="color: #888;">There are no active VPN clients.</i>
    </div>
  </div>
</template>


<script>
  import mixins from "@/utils/mixins"

  export default {
    name: "ClientList",
    mixins: [mixins.AppComponentMixin, mixins.PaginationMixin,],
    data() {
      return {
        clients: [],
        refreshTimeout: null,
      }
    },
    mounted() {
      this.getClients();
      this.refreshTimeout = setInterval(this.getClients, 5000);
    },
    destroyed() {
      clearInterval(this.refreshTimeout);
    },
    methods: {
      // getClients returns a paginated list of current OpenVPN client.
      getClients() {
        this.axios.get(`/admin/clients?search=${this.search}&page=${this.page}&size=${this.pageSize}`)
          .then(resp => {
            this.clients = resp.data.results;
            this.total = resp.data.count;
          });
      },

      // deleteClient deletes the client and kills the connection.
      deleteClient(client) {
        this.axios.delete(`/admin/clients/${client.id}/`).then(resp => {
          this.toastr.success("The client was disconnected.", "Disconnected");
          this.getClients();
        });
      },
    } // #Methods
  }
</script>
