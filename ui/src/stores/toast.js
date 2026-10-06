import { defineStore } from "pinia";
import { ref } from "vue";

const LIFETIME_MS = 4000;

/**
 * Short confirmations that something happened, shown in the corner and
 * dismissed on their own. Errors that belong to a field are shown against
 * the field instead.
 */
export const useToast = defineStore("toast", () => {
  const toasts = ref([]);
  let next = 1;

  function dismiss(id) {
    toasts.value = toasts.value.filter((toast) => toast.id !== id);
  }

  /**
   * @param {string} message
   * @param {"success" | "error"} [tone]
   */
  function notify(message, tone = "success") {
    const id = next++;
    toasts.value.push({ id, message, tone });
    setTimeout(() => dismiss(id), LIFETIME_MS);
  }

  return {
    toasts,
    dismiss,
    success: (message) => notify(message, "success"),
    error: (message) => notify(message, "error"),
  };
});
