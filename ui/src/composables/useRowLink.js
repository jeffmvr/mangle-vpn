import { useRouter } from "vue-router";

/**
 * Makes a whole table row open a page. The row's own link stays the way in
 * for keyboards and screen readers; this only widens the mouse target. A
 * click on another link or button in the row, or one that ends a text
 * selection, is left alone, and a modified click opens a new tab as the
 * link would.
 */
export function useRowLink() {
  const router = useRouter();

  return function openRow(event, to) {
    if (event.target.closest("a, button, input, select, label")) return;
    if (window.getSelection()?.toString()) return;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.button === 1) {
      window.open(router.resolve(to).href, "_blank", "noopener");
      return;
    }
    router.push(to);
  };
}
