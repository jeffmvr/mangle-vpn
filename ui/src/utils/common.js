

export default {
  // deleteFromArray is a helper function that removes an item from an array of
  // objects based on the value on the given object property.
  deleteFromObject: function (obj, key, value) {
    for (let i = 0; i < obj.length; i++) {
      if (obj[i][key] === value) {
        obj.splice(i, 1);
        return;
      }
    }
  },

  /**
   * Replaces any null values in the given object with empty strings.
   * @param {object} obj
   * @returns {null}
   */
  replaceNullWithEmptyString(obj) {
    for (let key in obj) {
      if (obj[key] === null) {
        obj[key] = "";
      }
    }
  }
}
