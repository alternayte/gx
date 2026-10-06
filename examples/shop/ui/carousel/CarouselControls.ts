import type { Mount } from "./CarouselControls.props";

// The island adds the previous and next buttons to a carousel. The slides
// are server HTML in a scroll container with CSS scroll snap, so with no
// script a user scrolls them with touch, a wheel or the arrow keys.
const mount: Mount = (root, props, ctx) => {
  const viewport = document.getElementById(props.viewport);
  if (!viewport) throw new Error(`no viewport with the id ${props.viewport}`);
  const c = props.classes;
  const vertical = props.orientation === "vertical";
  const slides = (): HTMLElement[] => [...viewport.children].filter((el): el is HTMLElement => el instanceof HTMLElement);

  root.className = c.root;
  const button = (label: string, text: string): HTMLButtonElement => {
    const b = root.appendChild(document.createElement("button"));
    b.type = "button";
    b.className = c.button;
    b.setAttribute("aria-label", label);
    b.setAttribute("aria-controls", props.viewport);
    b.textContent = text;
    return b;
  };
  const prev = button(props.previousLabel, vertical ? "↑" : "←");
  const status = root.appendChild(document.createElement("div"));
  status.className = c.status;
  status.setAttribute("aria-live", "polite");
  status.setAttribute("aria-atomic", "true");
  const next = button(props.nextLabel, vertical ? "↓" : "→");

  const offset = (el: HTMLElement): number => (vertical ? el.offsetTop - viewport.offsetTop : el.offsetLeft - viewport.offsetLeft);
  const position = (): number => (vertical ? viewport.scrollTop : viewport.scrollLeft);

  // current is the slide whose start is nearest to the scroll position.
  const current = (): number => {
    let best = 0;
    let distance = Infinity;
    slides().forEach((slide, i) => {
      const d = Math.abs(offset(slide) - position());
      if (d < distance) {
        best = i;
        distance = d;
      }
    });
    return best;
  };

  let shown = -1;
  const update = (): void => {
    const all = slides();
    const index = current();
    prev.disabled = index <= 0;
    next.disabled = index >= all.length - 1;
    all.forEach((slide, i) => slide.setAttribute("data-current", String(i === index)));
    if (index !== shown) {
      shown = index;
      status.textContent = props.statusText.replace("{n}", String(index + 1)).replace("{count}", String(all.length));
    }
  };

  const go = (index: number): void => {
    const all = slides();
    const slide = all[Math.max(0, Math.min(index, all.length - 1))];
    if (!slide) return;
    const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches;
    const to = offset(slide);
    viewport.scrollTo({ [vertical ? "top" : "left"]: to, behavior: reduce ? "auto" : "smooth" });
  };

  const listen = { signal: ctx.abort };
  prev.addEventListener("click", () => go(current() - 1), listen);
  next.addEventListener("click", () => go(current() + 1), listen);
  viewport.addEventListener("scroll", update, { ...listen, passive: true });
  // The arrow keys move one slide; the browser alone scrolls a few pixels.
  viewport.addEventListener(
    "keydown",
    (event) => {
      const back = vertical ? "ArrowUp" : "ArrowLeft";
      const forward = vertical ? "ArrowDown" : "ArrowRight";
      if (event.key !== back && event.key !== forward && event.key !== "Home" && event.key !== "End") return;
      event.preventDefault();
      if (event.key === "Home") go(0);
      else if (event.key === "End") go(slides().length - 1);
      else go(current() + (event.key === forward ? 1 : -1));
    },
    listen,
  );
  update();

  return () => {
    for (const slide of slides()) slide.removeAttribute("data-current");
    root.replaceChildren();
    root.className = "";
  };
};

export default mount;
