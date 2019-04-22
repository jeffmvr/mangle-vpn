<template>
  <div id="mainHeader" class="ui fixed inverted menu" style="padding: 0.66em 0;">
    <div class="ui container" style="width: 1024px;">
      <a href="/#/" class="header item">
        {{ store.appOrganization }} VPN
      </a>

      <!-- #LeftNavLinks -->
      <div class="left menu">

      </div><!-- #LeftNavLinks -->

      <!-- #RightNavLinks -->
      <div class="right menu" v-if="store.profile !== null">
        <a class="ui item simple dropdown">
          <i class="ui user icon"></i>&nbsp; {{ store.profile.email }}
          <i class="dropdown icon"></i>
          <div class="menu">
            <a href="/#/admin" class="item" v-if="store.profile.is_admin === true">
              <i class="cog icon"></i>
              Administration
            </a>
            <a href="/password" class="item">
              <i class="lock icon"></i>
              Reset Password
            </a>
            <a href="/logout" class="item">
              <i class="sign-out icon"></i>
              Sign Out
            </a>
            <div class="ui divider"></div>
            <div class="item disabled" style="text-align: center;">
              Mangle VPN v{{ store.appVersion }}
            </div>
          </div>
        </a>
      </div><!-- #RightNavLinks -->
    </div>
  </div>
</template>

<script>
  import BaseMixin from '@/components/mixins/BaseMixin'

  export default {
    name: "Header",
    mixins: [BaseMixin, ],
    mounted() {
      $('.ui.simple.dropdown').dropdown({
        on: "hover",
      });
    },
  }
</script>
