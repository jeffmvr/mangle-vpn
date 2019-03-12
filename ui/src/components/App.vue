<template>
  <div id="app">
    <app-header></app-header>

    <div id="mainContainer" class="ui container grid">
      <router-view/>
    </div>
  </div>
</template>


<script>
  import BaseMixin from '@/components/mixins/BaseMixin'
  import AppHeader from "@/components/common/Header"

  export default {
    name: "App",
    mixins: [
      BaseMixin,
    ],
    components: {
      AppHeader,
    },
    mounted() {
      // retrieves and saves the API information
      this.axios.get("/info")
        .then(resp => {
          this.store.appOrganization = resp.data.app_organization;
          this.store.appVersion = resp.data.app_version;
        });
    },
    watch: {
      /**
       * Watches for changes to the appOrganization and updates the browser title.
       * @param {string} newVal
       * @param {string} oldVal
       * @returns {null}
       */
      "store.appOrganization": function(newVal, oldVal) {
        document.title = `${this.store.appOrganization} VPN`;
      }
    }
  }
</script>


<style>
  .ui.container, #mainContainer {
    width: 1072px !important;
  }

  div#mainContainer {
    margin-top: 5em;
  }

  p.form-error {
    color: red;
    font-size: 0.9em;
  }

  .ui.vertical.menu .item > i.icon.left {
    float: none;
    margin: 0 0.35714286em 0 0;
  }

  div.form-actions {
    margin-top: 2.5em;
    text-align: center;
  }

  div#pageHeader {
    margin-bottom: 0.5em;
  }

  div.page-actions {
    margin-top: 0.7em;
    text-align: right;
  }

  div.header.item {
    background: #f0f0f0 !important;
  }

  div.page-buttons {
    text-align: right;
  }

  i.clickable {
    cursor: pointer;
  }

  div.empty {
    margin-top: 5em;
    text-align: center;
  }

  div.page-buttons > span {
    margin-right: 1em;
  }

  div.page-actions > div.ui.input.search {
    width: 55%;
  }

  div.page-actions > a.ui.button {
    margin-left: 1em;
  }

  .ui.body.segment {
    padding: 2em 2.5em;
  }

  .ui.title.segment {
      background: #e9e9e9;
  }

  input#code {
      letter-spacing: 0.33em;
      text-align: center;
  }
</style>
