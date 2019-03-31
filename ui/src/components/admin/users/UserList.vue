<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="six wide column">
        <h2>Users</h2>
      </div>
      <div class="ten wide column page-actions">
        <div class="ui icon mini input search">
          <i class="search icon"></i>
          <input class="input" type="text" placeholder="Search Users..." v-model="search" @keyup="performSearchOnType(getUsers)">
        </div>
        <a href="/#/admin/users/invite" class="ui tiny green button">
          <i class="plus icon"></i> Create Users
        </a>
      </div>
    </div><!-- #Header -->

    <table class="ui selectable table">
      <thead>
        <tr>
          <th style="width: 35%;">E-mail Address</th>
          <th style="width: 35%;">Group</th>
          <th style="width: 30%;">Last Login</th>
        </tr>
      </thead>

      <tbody>
        <tr v-for="user in users">
          <td>
            <a :href="`/#/admin/users/${user.id}`" :class="{ disabled: user.is_enabled === false }">
              {{ user.email }}
            </a>
          </td>
          <td>
            <a :href="`/#/admin/groups/${user.group.id}`">
              {{ user.group.name }}
            </a>
          </td>
          <td>{{ user.last_login | prettyDateTime }}</td>
        </tr>
      </tbody>
    </table>

    <div class="page-buttons">
      <span>{{ total }} Total</span>
      <button class="ui tiny button" @click="getUsers">
        <i class="icon sync"></i> Reload
      </button>
      <button :class="['ui tiny button', { 'disabled': !hasPrevPage }]" @click="prevPage(getUsers)">
        <i class="angle left icon"></i>
        Prev
      </button>
      <button :class="['ui tiny right button', { 'disabled': !hasNextPage }]" @click="nextPage(getUsers)">
        Next
        <i class="angle right icon"></i>
      </button>
    </div>
  </div>
</template>


<script>
  import BaseMixin from '@/components/mixins/BaseMixin'
  import PaginationMixin from '@/components/mixins/PaginationMixin'

  export default {
    name: "UserList",
    mixins: [
      BaseMixin,
      PaginationMixin,
    ],
    data() {
      return {
        users: [],
      }
    },
    mounted() {
      this.getUsers();
      $(".dropdown").dropdown();
    },
    methods: {
      // getUsers returns a list of all application users.
      getUsers() {
        this.axios.get(`/admin/users?search=${this.search}&page=${this.page}&size=${this.pageSize}`)
          .then(resp => {
            this.users = resp.data.results;
            this.total = resp.data.count;
          });
      },
    },
  }
</script>


<style>
  a.disabled {
    font-style: italic;
    text-decoration: line-through;
  }
</style>
