// Draws the enrolment QR code on the server-rendered two-factor setup page.
// It lives in a file rather than inline so that the page's Content Security
// Policy can refuse every inline script. The URI to encode is carried on the
// canvas itself.
(function () {
  var canvas = document.getElementById("qr");
  if (!canvas || typeof QRious === "undefined") return;

  new QRious({ element: canvas, value: canvas.dataset.value, size: 360 });
})();
