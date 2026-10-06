import axios from "axios";

/**
 * The HTTP client every view talks to.
 *
 * Two things here are load-bearing and must not drift from the server:
 * the CSRF contract, and what the application does when the server says
 * a request is not allowed.
 */
const api = axios.create({
  baseURL: "/api",
  withCredentials: true,
});

/**
 * Returns the value of a cookie, or an empty string when it is unset.
 *
 * @param {string} name
 * @returns {string}
 */
export function cookie(name) {
  const match = document.cookie.match(new RegExp(`(^|;\\s*)${name}=([^;]*)`));
  return match ? decodeURIComponent(match[2]) : "";
}

// The server issues a readable csrftoken cookie and expects it echoed in
// the X-CSRFToken header. A cross-origin page cannot read the cookie, which
// is what makes the pair meaningful.
api.interceptors.request.use((config) => {
  const token = cookie("csrftoken");
  if (token) {
    config.headers["X-CSRFToken"] = token;
  }
  return config;
});

/**
 * Where to send someone the server has refused.
 *
 * The server names the reason it refused, so go straight to the page that
 * resolves it instead of bouncing off "/" and letting the server work it
 * out. That also matters during development, where Vite serves "/" and
 * would hand back the same application that was just refused.
 */
const refusals = {
  NotLoggedIn: "/login",
  MfaNotEnabled: "/mfa/setup",
  MfaNotConfirmed: "/mfa",
  PasswordChangeRequired: "/password",
};

api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status;

    if (status === 401 || status === 403) {
      const reason = error.response?.data?.detail;
      const destination = refusals[reason] ?? "/";

      // Never navigate to the page we are already on; that is a reload
      // loop rather than a redirect.
      if (window.location.pathname !== destination) {
        window.location.href = destination;
      }
    }

    return Promise.reject(error);
  },
);

/**
 * Pulls field errors out of a rejected request.
 *
 * The server answers a failed validation with {field: ["message", ...]},
 * which is what the forms render against.
 *
 * @param {unknown} error
 * @returns {Record<string, string>}
 */
export function fieldErrors(error) {
  const data = error?.response?.data;
  if (!data || typeof data !== "object") return {};

  return Object.fromEntries(
    Object.entries(data).map(([field, messages]) => [
      field,
      Array.isArray(messages) ? messages[0] : String(messages),
    ]),
  );
}

/**
 * A sentence describing why a request failed, for a toast.
 *
 * @param {unknown} error
 * @param {string} [fallback]
 * @returns {string}
 */
export function errorMessage(error, fallback = "Something went wrong. Please try again.") {
  const detail = error?.response?.data?.detail;
  return typeof detail === "string" && detail ? detail : fallback;
}

export default api;
