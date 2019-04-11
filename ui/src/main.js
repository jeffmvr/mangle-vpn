import Vue from 'vue'
import axios from 'axios'
import toastr from 'toastr'
import VueAxios from 'vue-axios'
import VueCookie from 'vue-cookie'
import App from '@/components/App'
import router from '@/routes'
import '@/utils/filters'
import '@/utils/csrf'

Vue.config.productionTip = false;

// Vue plugin registration
Vue.use(VueCookie);
Vue.use(VueAxios, axios);

axios.defaults.baseURL = "/api";

// default response error handlers
axios.interceptors.response.use(resp => {
  return resp;
}, err => {
  switch(err.response.status) {
    case 401:
    case 403:
      // an HTTP 401 response indicates that the user has not yet authenticated
      // or are attempting to access something they shouldn't... in which case
      // they will be redirected to the logout page
      window.location.href = "/";
      break;
    case 404:
      // an HTTP 404 response indicates the API resource the user is attempting
      // to access cannot be found, in which case they are redirected back to
      // their profile
      toastr.error("The resource could not be found.", "Not Found");
      break;
    default:
      if (err.response.status >= 500) {
        toastr.error("A server error occurred.", "Server Error");
      }
  }

  return Promise.reject(err);
});

/* eslint-disable no-new */
new Vue({
  el: '#app',
  router,
  components: { App },
  template: '<App/>'
});
