<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="six wide column">
        <h2>Groups</h2>
      </div>
      <div class="ten wide column page-actions">
        <div class="ui icon mini input search">
          <i class="search icon"></i>
          <input class="input" type="text" placeholder="Search Groups..." v-model="search" @keyup="performSearchOnType(getGroups)">
        </div>
        <a href="/#/admin/groups/new" class="ui tiny green button">
          <i class="plus icon"></i> Create Group
        </a>
      </div>
    </div><!-- #Header -->

    <!-- #GroupsTable -->
    <table class="ui selectable table">
      <thead>
        <tr>
          <th style="width: 40%;">Name</th>
          <th style="width: 60%;">Description</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="group in groups">
          <td>
            <a :href="`/#/admin/groups/${group.id}`">
              {{ group.name }}
            </a>
          </td>
          <td>{{ group.description }}</td>
        </tr>
      </tbody>
    </table><!-- #GroupsTable -->

    <!-- #Pagination -->
    <div class="page-buttons">
      <span>{{ total }} Total</span>
      <button class="ui tiny button" @click="getGroups">
        <i class="icon sync"></i> Reload
      </button>
      <button :class="['ui tiny button', { 'disabled': !hasPrevPage }]" @click="prevPage(getGroups)">
        <i class="angle left icon"></i>
        Prev
      </button>
      <button :class="['ui tiny right button', { 'disabled': !hasNextPage }]" @click="nextPage(getGroups)">
        Next
        <i class="angle right icon"></i>
      </button>
    </div><!-- #Pagination -->
  </div>
</template>


<script>
  import BaseMixin from '@/components/mixins/BaseMixin'
  import PaginationMixin from '@/components/mixins/PaginationMixin'

  export default {
    name: "GroupList",
    mixins: [
      BaseMixin,
      PaginationMixin,
    ],
    data() {
      return {
        groups: [],
      }
    },
    mounted() {
      this.getGroups();
    },
    methods: {
      // getGroups retrieves a paginated list of groups
      getGroups() {
        this.axios.get(`/admin/groups?search=${this.search}&page=${this.page}&size=${this.pageSize}`)
          .then(resp => {
            this.groups = resp.data.results;
            this.total = resp.data.count;
          });
      }
    }
  }
</script>
