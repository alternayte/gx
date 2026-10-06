import type { Mount } from "./ComboboxInput.props";

// The island turns one native select into a combobox: a text input that
// filters the options, with a list below it. The select stays the form
// control: the island writes the chosen value into it and sends its change
// event. With no script the page shows the select.
const mount: Mount = (root, props, ctx) => {
  const select = document.getElementById(props.select);
  if (!(select instanceof HTMLSelectElement)) {
    throw new Error(`no select with the id ${props.select}`);
  }
  const c = props.classes;
  const hadClass = select.className;
  select.className = c.hidden;
  select.tabIndex = -1;
  select.setAttribute("aria-hidden", "true");

  root.className = c.root;
  const input = root.appendChild(document.createElement("input"));
  input.type = "text";
  input.className = c.input;
  input.placeholder = props.placeholder;
  input.autocomplete = "off";
  input.spellcheck = false;
  input.setAttribute("role", "combobox");
  input.setAttribute("aria-autocomplete", "list");
  input.setAttribute("aria-expanded", "false");
  const listId = props.select + "-list";
  input.setAttribute("aria-controls", listId);
  const label = select.getAttribute("aria-label");
  if (label) input.setAttribute("aria-label", label);
  // A label for the select names the combobox too.
  const labels = [...(select.labels ?? [])];
  if (labels.length > 0) {
    labels.forEach((l, i) => (l.id ||= `${props.select}-label-${i}`));
    input.setAttribute("aria-labelledby", labels.map((l) => l.id).join(" "));
  }
  input.disabled = select.disabled;

  const list = root.appendChild(document.createElement("ul"));
  list.id = listId;
  list.className = c.list;
  list.hidden = true;
  list.setAttribute("role", "listbox");
  if (label) list.setAttribute("aria-label", label);
  const empty = root.appendChild(document.createElement("div"));
  empty.className = c.empty;
  empty.hidden = true;
  empty.textContent = props.emptyText;

  type Option = { value: string; label: string; disabled: boolean };
  const options: Option[] = [...select.options]
    .filter((o) => o.value !== "")
    .map((o) => ({ value: o.value, label: o.label, disabled: o.disabled }));

  let open = false;
  let shown: Option[] = [];
  let active = -1;

  const chosen = (): Option | undefined => options.find((o) => o.value === select.value);

  const render = (): void => {
    list.replaceChildren();
    shown.forEach((option, i) => {
      const item = list.appendChild(document.createElement("li"));
      item.id = `${listId}-${i}`;
      item.className = c.option;
      item.textContent = option.label;
      item.setAttribute("role", "option");
      item.setAttribute("data-value", option.value);
      item.setAttribute("aria-selected", String(option.value === select.value));
      item.setAttribute("data-active", String(i === active));
      if (option.disabled) item.setAttribute("aria-disabled", "true");
    });
    list.hidden = !open || shown.length === 0;
    empty.hidden = !open || shown.length > 0;
    input.setAttribute("aria-expanded", String(open));
    if (open && active >= 0) {
      input.setAttribute("aria-activedescendant", `${listId}-${active}`);
      list.children[active]?.scrollIntoView({ block: "nearest" });
    } else {
      input.removeAttribute("aria-activedescendant");
    }
  };

  // filter shows the options whose label holds the text. With no text, or
  // with the text of the chosen option, it shows them all.
  const filter = (): void => {
    const text = input.value.trim().toLowerCase();
    const all = text === "" || text === (chosen()?.label ?? "").toLowerCase();
    shown = all ? options : options.filter((o) => o.label.toLowerCase().includes(text));
  };

  const show = (): void => {
    if (open) return;
    open = true;
    filter();
    active = shown.findIndex((o) => o.value === select.value);
    render();
  };

  const hide = (): void => {
    if (!open) return;
    open = false;
    active = -1;
    // Text that names no option goes back to the chosen one.
    input.value = chosen()?.label ?? "";
    render();
  };

  const choose = (option: Option): void => {
    if (option.disabled) return;
    select.value = option.value;
    input.value = option.label;
    open = false;
    active = -1;
    render();
    select.dispatchEvent(new Event("input", { bubbles: true }));
    select.dispatchEvent(new Event("change", { bubbles: true }));
  };

  // move goes to the next option that is not disabled, in one direction.
  const move = (from: number, step: number): void => {
    for (let i = from; i >= 0 && i < shown.length; i += step) {
      if (!shown[i].disabled) {
        active = i;
        break;
      }
    }
    render();
  };

  const listen = { signal: ctx.abort };
  input.addEventListener(
    "input",
    () => {
      open = true;
      filter();
      active = shown.findIndex((o) => !o.disabled);
      render();
    },
    listen,
  );
  input.addEventListener("click", show, listen);
  input.addEventListener(
    "keydown",
    (event) => {
      switch (event.key) {
        case "ArrowDown":
          event.preventDefault();
          if (!open) show();
          move(active + 1, 1);
          break;
        case "ArrowUp":
          event.preventDefault();
          if (!open) show();
          move(active < 0 ? shown.length - 1 : active - 1, -1);
          break;
        case "Home":
          if (!open) return;
          event.preventDefault();
          move(0, 1);
          break;
        case "End":
          if (!open) return;
          event.preventDefault();
          move(shown.length - 1, -1);
          break;
        case "Enter":
          if (!open || active < 0) return;
          event.preventDefault();
          choose(shown[active]);
          break;
        case "Escape":
          if (!open) return;
          event.preventDefault();
          hide();
          break;
      }
    },
    listen,
  );
  // The pointer goes down on an option before the input loses the focus.
  list.addEventListener("mousedown", (event) => event.preventDefault(), listen);
  list.addEventListener(
    "click",
    (event) => {
      const item = (event.target as Element).closest("[role=option]");
      const option = item ? shown[[...list.children].indexOf(item)] : undefined;
      if (option) choose(option);
    },
    listen,
  );
  input.addEventListener("blur", hide, listen);
  // A reset of the form, or code of the page, can change the select.
  select.addEventListener(
    "change",
    () => {
      if (!open) input.value = chosen()?.label ?? "";
    },
    listen,
  );
  select.form?.addEventListener("reset", () => setTimeout(() => (input.value = chosen()?.label ?? "")), listen);
  input.value = chosen()?.label ?? "";
  render();

  return () => {
    select.className = hadClass;
    select.removeAttribute("tabindex");
    select.removeAttribute("aria-hidden");
    root.replaceChildren();
    root.className = "";
  };
};

export default mount;
