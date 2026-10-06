import type { Mount } from "./ResizableDrag.props";

// The island makes one handle between two panels movable: with the pointer
// and with the arrow keys. The panels are server HTML with their first
// sizes, so with no script the page shows a fixed split.
const mount: Mount = (root, props, ctx) => {
  const handle = document.getElementById(props.handle);
  if (!handle) throw new Error(`no handle with the id ${props.handle}`);
  const before = handle.previousElementSibling;
  const after = handle.nextElementSibling;
  const group = handle.parentElement;
  if (!(before instanceof HTMLElement) || !(after instanceof HTMLElement) || !group) {
    throw new Error("a handle needs a panel before it and a panel after it");
  }
  const vertical = group.getAttribute("data-orientation") === "vertical";
  root.className = props.gripClass;
  root.setAttribute("aria-hidden", "true");

  const size = (el: HTMLElement): number => (vertical ? el.getBoundingClientRect().height : el.getBoundingClientRect().width);
  const limit = (el: HTMLElement, name: string, fallback: number): number => {
    const v = Number(el.getAttribute(name));
    return Number.isFinite(v) && el.hasAttribute(name) ? v : fallback;
  };

  // A size is a part of the room that the panels of the group share, in
  // percent. The handles take none of that room.
  const panels = (): HTMLElement[] =>
    [...group.children].filter((el): el is HTMLElement => el instanceof HTMLElement && el.getAttribute("data-slot") === "resizable-panel");
  const room = (): number => panels().reduce((sum, el) => sum + size(el), 0);
  const total = (): number => ((size(before) + size(after)) / room()) * 100;
  const share = (): number => (size(before) / room()) * 100;
  // Each panel gets its measured size as its weight, so a panel with no
  // size from the server keeps its place when a different one changes.
  const weigh = (): void => {
    const all = panels();
    const sizes = all.map(size);
    const sum = sizes.reduce((a, b) => a + b, 0) || 1;
    all.forEach((el, i) => (el.style.flex = `${(sizes[i] / sum) * 100} 1 0%`));
  };

  // set gives the first panel a size in percent of the group, inside the
  // limits of both panels. The second panel takes the rest of the two.
  const set = (percent: number): void => {
    const both = total();
    const min = Math.max(limit(before, "data-min-size", 0), both - limit(after, "data-max-size", 100));
    const max = Math.min(limit(before, "data-max-size", 100), both - limit(after, "data-min-size", 0));
    const value = Math.max(min, Math.min(max, percent));
    before.style.flex = `${value} 1 0%`;
    after.style.flex = `${both - value} 1 0%`;
    handle.setAttribute("aria-valuenow", String(Math.round(value)));
    handle.setAttribute("aria-valuemin", String(Math.round(min)));
    handle.setAttribute("aria-valuemax", String(Math.round(max)));
    handle.dispatchEvent(new CustomEvent("resize-panels", { bubbles: true, detail: { before: value, after: both - value } }));
  };

  const listen = { signal: ctx.abort };
  let dragging = false;
  handle.addEventListener(
    "pointerdown",
    (event) => {
      dragging = true;
      handle.setPointerCapture(event.pointerId);
      handle.setAttribute("data-dragging", "true");
      event.preventDefault();
    },
    listen,
  );
  handle.addEventListener(
    "pointermove",
    (event) => {
      if (!dragging) return;
      const box = group.getBoundingClientRect();
      const start = vertical ? before.getBoundingClientRect().top : before.getBoundingClientRect().left;
      const at = vertical ? event.clientY : event.clientX;
      set(((at - start) / (vertical ? box.height : box.width)) * 100);
    },
    listen,
  );
  const stop = (): void => {
    dragging = false;
    handle.removeAttribute("data-dragging");
  };
  handle.addEventListener("pointerup", stop, listen);
  handle.addEventListener("pointercancel", stop, listen);
  handle.addEventListener(
    "keydown",
    (event) => {
      const back = vertical ? "ArrowUp" : "ArrowLeft";
      const forward = vertical ? "ArrowDown" : "ArrowRight";
      switch (event.key) {
        case back:
          set(share() - props.step);
          break;
        case forward:
          set(share() + props.step);
          break;
        case "Home":
          set(0);
          break;
        case "End":
          set(100);
          break;
        default:
          return;
      }
      event.preventDefault();
    },
    listen,
  );
  // The first sizes come from the server; the handle reports them.
  weigh();
  set(share());

  return () => {
    root.className = "";
  };
};

export default mount;
