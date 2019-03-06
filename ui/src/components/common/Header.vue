<template>
  <div class="ui fixed inverted menu" style="padding: 0.66em 0;">
    <div class="ui container" style="width: 1024px;">
      <a href="/#/" class="header item">
        {{ store.appOrganization }} VPN
      </a>

      <!-- #LeftNavLinks -->
      <div class="left menu" v-if="store.profile !== null">
        <a href="/#/" :class="['item', {'active': store.activePage === ''}]">
          Devices
        </a>
        <a href="/#/admin" :class="['item', {'active': store.activePage === 'admin'}]" v-if="store.profile.is_admin">
          Administration
        </a>
        <a href="/logout" class="item">
          Logout
        </a>
      </div><!-- #LeftNavLinks -->

      <!-- #RightNavLinks -->
      <div class="right menu" v-if="store.profile !== null">
        <div class="item">
          {{ store.profile.email }}
        </div>
      </div><!-- #RightNavLinks -->
    </div>
  </div>
</template>

<script>
  import BaseMixin from '@/components/mixins/BaseMixin'

  export default {
    name: "Header",
    mixins: [BaseMixin, ],
    watch: {
      /**
       * Updates the store's ``activePage`` value whenever the route changes.
       * @param {string} to
       * @param {string} from
       * @returns {null}
       */
      $route (to, from) {
        this.setActivePage();
      },
    },
  }
</script>
