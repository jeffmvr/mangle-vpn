import toastr from "toastr";
import store from '@/store';


export default {
  data() {
    return {
      toastr: toastr,
      store: store,
    }
  },
  mounted() {
    this.setActivePage();
    this.getProfile();
  },
  methods: {
    /**
     * Retrieves and stores the current user's profile.
     * @returns {null}
     */
    getProfile() {
      this.axios.get("/profile")
        .then(resp => {
          this.store.profile = resp.data;
          this.store.appInitialized = true;
        });
    },

    /**
     * Hides all of the application modals.
     * @returns {null}
     */
    hideModals() {
      $(".ui.modal").modal("hide all")
    },

    /**
     * Displays the modal with the given ID attribute and modal options.
     * @param {string} modalID
     * @param {object | optional} options
     * @returns {boolean}
     */
    showModal(modalID, options) {
      let modal = this[modalID];

      // if the modal does not exist as a data property, then attempt to get it
      // from the DOM. If the modal still does not exist, then do nothing
      if (modal === undefined) {
        modal = $(`#${modalID}`);

        if (modal === undefined) {
          console.error(`failed to find modal with ID: ${modalID}`);
          return false;
        }
      }

      // only add the modal options if it is an object
      if (options instanceof Object) {
        modal.modal(options);
      }

      modal.modal("show");
      return true;
    },

    /**
     * Sets the store's active page based on the current route.
     * @returns {null}
     */
    setActivePage() {
      this.store.activePage = this.$router.currentRoute.path.split("/")[1];
    },
  },
}
