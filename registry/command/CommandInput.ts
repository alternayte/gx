import type { Mount } from "./CommandInput.props";

// The island adds a search input to a list of commands that the server
// rendered. It filters the items, moves an active item with the arrow keys
// and runs the active item on Enter. With no script the page shows the
// whole list, and each item is a link or a button that works by itself.
const mount: Mount = (root, props, ctx) => {
  const list = document.getElementById(props.list);
  if (!list) throw new Error(`no list with the id ${props.list}`);
  const empty = props.empty ? document.getElementById(props.empty) : null;
  const c = props.classes;

  root.className = c.root;
  const input = root.appendChild(document.createElement("input"));
  input.type = "text";
  input.className = c.input;
  input.placeholder = props.placeholder;
  input.autocomplete = "off";
  input.spellcheck = false;
  input.setAttribute("role", "combobox");
  input.setAttribute("aria-autocomplete", "list");
  input.setAttribute("aria-expanded", "true");
  input.setAttribute("aria-controls", props.list);
  input.setAttribute("aria-label", props.label);

  const items = [...list.querySelectorAll<HTMLElement>("[data-command-item]")];
  items.forEach((item, i) => {
    item.id ||= `${props.list}-item-${i}`;
    // The input keeps the focus; the items are reached with the arrow keys.
    item.tabIndex = -1;
  });
  const groups = [...list.querySelectorAll<HTMLElement>("[data-command-group]")];
  let active: HTMLElement | null = null;

  const enabled = (item: HTMLElement): boolean => !item.hidden && item.getAttribute("aria-disabled") !== "true";
  const visible = (): HTMLElement[] => items.filter(enabled);

  const setActive = (item: HTMLElement | null): void => {
    active = item;
    for (const other of items) other.setAttribute("data-active", String(other === item));
    if (item) {
      input.setAttribute("aria-activedescendant", item.id);
      item.scrollIntoView({ block: "nearest" });
    } else {
      input.removeAttribute("aria-activedescendant");
    }
  };

  // filter shows each item whose text or keywords hold every word of the
  // input.
  const filter = (): void => {
    const words = input.value.toLowerCase().split(/\s+/).filter(Boolean);
    for (const item of items) {
      const text = `${item.getAttribute("data-label") ?? item.textContent ?? ""} ${item.getAttribute("data-keywords") ?? ""}`.toLowerCase();
      item.hidden = !words.every((word) => text.includes(word));
    }
    for (const group of groups) {
      group.hidden = ![...group.querySelectorAll<HTMLElement>("[data-command-item]")].some((item) => !item.hidden);
    }
    const shown = visible();
    if (empty) empty.hidden = items.some((item) => !item.hidden);
    setActive(shown[0] ?? null);
  };

  const listen = { signal: ctx.abort };
  input.addEventListener("input", filter, listen);
  input.addEventListener(
    "keydown",
    (event) => {
      const shown = visible();
      const at = active ? shown.indexOf(active) : -1;
      switch (event.key) {
        case "ArrowDown":
          event.preventDefault();
          setActive(shown[Math.min(at + 1, shown.length - 1)] ?? null);
          break;
        case "ArrowUp":
          event.preventDefault();
          setActive(shown[Math.max(at - 1, 0)] ?? null);
          break;
        case "Home":
          event.preventDefault();
          setActive(shown[0] ?? null);
          break;
        case "End":
          event.preventDefault();
          setActive(shown[shown.length - 1] ?? null);
          break;
        case "Enter":
          if (!active) return;
          event.preventDefault();
          // A link goes to its address and a button sends its click.
          active.click();
          break;
      }
    },
    listen,
  );
  list.addEventListener(
    "pointermove",
    (event) => {
      const item = (event.target as Element).closest<HTMLElement>("[data-command-item]");
      if (item && item !== active && enabled(item)) setActive(item);
    },
    listen,
  );
  // An item tells the page that the user ran it. The page can listen on
  // the command element.
  list.addEventListener(
    "click",
    (event) => {
      const item = (event.target as Element).closest<HTMLElement>("[data-command-item]");
      if (!item || !enabled(item)) return;
      item.dispatchEvent(new CustomEvent("command-select", { bubbles: true, detail: { value: item.getAttribute("data-value") ?? "" } }));
    },
    listen,
  );
  filter();

  return () => {
    for (const item of items) {
      item.hidden = false;
      item.removeAttribute("tabindex");
      item.removeAttribute("data-active");
    }
    for (const group of groups) group.hidden = false;
    if (empty) empty.hidden = true;
    root.replaceChildren();
    root.className = "";
  };
};

export default mount;
