// Setup's last step creates keys, which takes a few seconds: say so, and
// keep the form from being sent twice.
document.querySelectorAll("form[data-busy-label]").forEach((form) => {
  form.addEventListener("submit", () => {
    const button = form.querySelector("button[type=submit]");
    if (!button) return;
    button.textContent = form.dataset.busyLabel;
    button.setAttribute("aria-disabled", "true");
    form.querySelectorAll("a.button").forEach((link) => link.setAttribute("aria-disabled", "true"));
  });
});
