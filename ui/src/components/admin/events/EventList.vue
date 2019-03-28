<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="seven wide column">
        <h2>Events</h2>
      </div>
      <div class="nine wide column page-actions">
        <div class="ui icon mini input search">
          <i class="search icon"></i>
          <input class="input" type="text" placeholder="Search Events..." v-model="search" @keyup="performSearchOnType(getEvents)">
        </div>
      </div>
    </div><!-- #Header -->

    <table class="ui selectable table">
      <thead>
        <tr>
          <th style="width: 15%;">Date/Time</th>
          <th style="width: 22%;">Name</th>
          <th style="width: 32%;">User</th>
          <th style="width: 31%;">Detail</th>
        </tr>
      </thead>

      <tbody>
        <tr v-for="event in events">
          <td>{{ event.created_at | prettyDateTime }}</td>
          <td>{{ event.name }}</td>
          <td>
            <a :href="`/#/admin/users/${event.user.id}`">
              {{ event.user.email }}
            </a>
          </td>
          <td>{{ event.detail }}</td>
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
</template>


<script>
  import mixins from "@/utils/mixins"

  export default {
    name: "EventList",
    mixins: [
      mixins.AppComponentMixin,
      mixins.PaginationMixin,
    ],
    data() {
      return {
        events: [],
        refreshTimeout: null,
      }
    },
    mounted() {
      this.getEvents();
      this.refreshTimeout = setInterval(this.getEvents, 10000);
    },
    destroyed() {
      clearInterval(this.refreshTimeout);
    },
    methods: {
      // getEvents returns a paginated list of application events.
      getEvents() {
        this.axios.get(`/admin/events?search=${this.search}&page=${this.page}&size=${this.pageSize}`)
          .then(resp => {
            this.events = resp.data.results;
            this.total = resp.data.count;
          });
      },
    } // #Methods
  }
</script>
