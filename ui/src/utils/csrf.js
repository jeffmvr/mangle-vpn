import axios from "axios";
import VueCookie from "vue-cookie";


// Adds the CSRF cookie to the request headers.
axios.interceptors.request.use(config => {
  config.headers["X-CSRFToken"] = VueCookie.get("csrftoken");
  return config;
});
