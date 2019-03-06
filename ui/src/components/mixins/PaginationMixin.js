

export default {
  data() {
    return {
      page: 1,
      pageSize: 15,
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

    /**
     * Retrieves the next page of items using the given function.
     * @param {function} handler
     * @returns {null}
     */
    nextPage(handler) {
      if (this.page < this.getTotalPages()) {
        this.page++;
        handler();
      }
    },

    /**
     * Retrieves the previous page of items using the given function.
     * @param {function} handler
     * @returns {null}
     */
    prevPage(handler) {
      if (this.page > 1) {
        this.page--;
        handler();
      }
    },

    /**
     * Returns the total number of pages.
     * @returns {number}
     */
    getTotalPages() {
      return Math.ceil(this.total/this.pageSize);
    }
  },

  computed: {
    /**
     * Returns whether there is a next page of items.
     * @returns {boolean}
     */
    hasNextPage: function() {
      return this.page < this.getTotalPages();
    },

    /**
     * Returns whether there is a previous page of items.
     * @returns {boolean}
     */
    hasPrevPage: function() {
      return this.page > 1;
    },
  }
}
