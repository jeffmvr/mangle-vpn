import toastr from 'toastr'
import store from '@/store'


export default {
  AppComponentMixin: {
    data() {
      return {
        toastr: toastr,
        store: store,
      }
    },
    methods: {
      // hideModals hides all of the application modals.
      hideModals() {
        $(".ui.modal").modal("hide all")
      },

      // showModal shows the modal with the given ID with the given options.
      showModal(modalID, options) {
        this.hideModals();
        // Add options only if provided to prevent overwriting any previously
        // set options
        if (options !== undefined) {
          this[modalID].modal(options);
        }
        this[modalID].modal("show");
      },

      // getProfile retrieves and sets the user's profile.
      getProfile() {
        this.axios.get("/profile").then(resp => {
          this.store.profile = resp.data;
          this.store.appInitialized = true;
        });
      },
    }, // #Methods
  }, // #AppComponentMixin

  //
  // PaginationMixin
  //
  PaginationMixin: {
    data() {
      return {
        page: 1,
        pageSize: 10,
        search: "",
        total: 0,
      }
    },
    methods: {
      // performSearchOnType resets the page to 1 and performs the search using
      // the given function.
      performSearchOnType(handler) {
        this.page = 1;
        handler();
      },

      // nextPage loads the next page of results.
      nextPage(handler) {
        if (this.hasNextPage) {
          this.page++;
          handler();
        }
      },

      // prevPage loads the previous page of results.
      prevPage(handler) {
        if (this.hasPrevPage) {
          this.page--;
          handler();
        }
      },

      // getTotalPages returns the total number of pages.
      getTotalPages() {
        return Math.ceil(this.total/this.pageSize);
      }
    }, // #Methods
    computed: {
      // hasNextPage returns true if there is a next page.
      hasNextPage: function() {
        return this.page < this.getTotalPages();
      },

      // hasPrevPage returns true if there is a previous page.
      hasPrevPage: function() {
        return this.page > 1;
      },
    }
  }
}
