import type { Mount, Props } from "./CalendarGrid.props";

// A day as the count of days from 1970-01-01, in UTC. A calendar date has no
// time zone, so every step here is whole days.
type Day = number;

const dayMs = 86400000;

const toDay = (iso: string): Day | null => {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return null;
  const ms = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  const date = new Date(ms);
  // 2026-02-31 is not a date.
  return date.getUTCDate() === Number(m[3]) ? ms / dayMs : null;
};

const toISO = (day: Day): string => new Date(day * dayMs).toISOString().slice(0, 10);

const parts = (day: Day): { year: number; month: number; date: number } => {
  const d = new Date(day * dayMs);
  return { year: d.getUTCFullYear(), month: d.getUTCMonth(), date: d.getUTCDate() };
};

const fromParts = (year: number, month: number, date: number): Day => Date.UTC(year, month, date) / dayMs;

const daysIn = (year: number, month: number): number => new Date(Date.UTC(year, month + 1, 0)).getUTCDate();

// addMonths moves a day by whole months and keeps the day of the month
// where the new month has it: 31 January plus one month is 28 February.
const addMonths = (day: Day, count: number): Day => {
  const { year, month, date } = parts(day);
  const first = new Date(Date.UTC(year, month + count, 1));
  const y = first.getUTCFullYear();
  const m = first.getUTCMonth();
  return fromParts(y, m, Math.min(date, daysIn(y, m)));
};

const today = (): Day => {
  const now = new Date();
  return fromParts(now.getFullYear(), now.getMonth(), now.getDate());
};

const el = <K extends keyof HTMLElementTagNameMap>(tag: K, className: string, parent?: Element): HTMLElementTagNameMap[K] => {
  const node = document.createElement(tag);
  if (className) node.className = className;
  parent?.appendChild(node);
  return node;
};

// The island draws one month as a grid of days. The native date input with
// the id props.input stays the form control: the island writes the chosen
// day into it and sends its change event. With no script the page shows the
// date input of the browser.
const mount: Mount = (root, props: Props, ctx) => {
  const input = document.getElementById(props.input);
  if (!(input instanceof HTMLInputElement)) {
    throw new Error(`no input with the id ${props.input}`);
  }
  const c = props.classes;
  const locale = props.locale || document.documentElement.lang || undefined;
  const min = toDay(input.min);
  const max = toDay(input.max);
  const allowed = (day: Day): boolean => (min === null || day >= min) && (max === null || day <= max);

  let selected = toDay(input.value);
  // focus is the day that takes the Tab key: the chosen day, or the first
  // day of the month in the props, or today.
  let focus: Day = selected ?? toDay(props.month + "-01") ?? today();
  const hadClass = input.className;
  input.className = c.hidden;
  input.tabIndex = -1;

  root.className = c.root;
  const header = el("div", c.header, root);
  const prev = el("button", c.nav, header);
  prev.type = "button";
  prev.setAttribute("aria-label", props.previousLabel);
  prev.textContent = "‹";
  const title = el("div", c.title, header);
  title.id = props.input + "-month";
  title.setAttribute("aria-live", "polite");
  const next = el("button", c.nav, header);
  next.type = "button";
  next.setAttribute("aria-label", props.nextLabel);
  next.textContent = "›";

  const table = el("table", c.table, root);
  table.setAttribute("role", "grid");
  table.setAttribute("aria-labelledby", title.id);
  const head = el("tr", "", el("thead", "", table));
  const body = el("tbody", "", table);

  const monthName = new Intl.DateTimeFormat(locale, { month: "long", year: "numeric", timeZone: "UTC" });
  const dayName = new Intl.DateTimeFormat(locale, { dateStyle: "full", timeZone: "UTC" });
  const weekdayShort = new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: "UTC" });
  const weekdayLong = new Intl.DateTimeFormat(locale, { weekday: "long", timeZone: "UTC" });
  // 4 January 1970 is a Sunday.
  for (let i = 0; i < 7; i++) {
    const date = new Date((3 + props.weekStart + i) * dayMs);
    const th = el("th", c.weekday, head);
    th.scope = "col";
    th.abbr = weekdayLong.format(date);
    th.textContent = weekdayShort.format(date);
  }

  const render = (moveFocus: boolean): void => {
    const { year, month } = parts(focus);
    const first = fromParts(year, month, 1);
    title.textContent = monthName.format(new Date(first * dayMs));
    // The weekday of the first day, counted from the first day of the week.
    const offset = (new Date(first * dayMs).getUTCDay() - props.weekStart + 7) % 7;
    const count = daysIn(year, month);
    const now = today();
    body.replaceChildren();
    let row: HTMLTableRowElement | null = null;
    for (let cell = 0; cell < Math.ceil((offset + count) / 7) * 7; cell++) {
      if (cell % 7 === 0) row = el("tr", "", body);
      const td = el("td", c.cell, row!);
      const date = cell - offset + 1;
      if (date < 1 || date > count) continue;
      const day = first + date - 1;
      const button = el("button", c.day, td);
      button.type = "button";
      button.textContent = String(date);
      button.dataset.day = toISO(day);
      button.setAttribute("aria-label", dayName.format(new Date(day * dayMs)));
      button.tabIndex = day === focus ? 0 : -1;
      button.disabled = !allowed(day);
      // The cell of a grid holds the selected state.
      td.setAttribute("aria-selected", String(day === selected));
      if (day === selected) button.className = c.day + " " + c.selected;
      if (day === now) {
        button.setAttribute("aria-current", "date");
        if (day !== selected) button.className = c.day + " " + c.today;
      }
    }
    if (moveFocus) body.querySelector<HTMLButtonElement>('button[tabindex="0"]')?.focus();
  };

  const select = (day: Day): void => {
    if (!allowed(day)) return;
    selected = day;
    focus = day;
    input.value = toISO(day);
    render(true);
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.dispatchEvent(new Event("change", { bubbles: true }));
    // A date picker shows the day in its trigger and closes its popover.
    show();
    if (popover?.matches(":popover-open")) popover.hidePopover();
  };

  // show writes the chosen day into the display element of a date picker.
  const display = props.display ? document.getElementById(props.display) : null;
  const popover = props.popover ? document.getElementById(props.popover) : null;
  const show = (): void => {
    if (!display || selected === null) return;
    display.textContent = dayName.format(new Date(selected * dayMs));
    display.setAttribute("data-empty", "false");
  };

  const options = { signal: ctx.abort };
  prev.addEventListener("click", () => { focus = addMonths(focus, -1); render(false); }, options);
  next.addEventListener("click", () => { focus = addMonths(focus, 1); render(false); }, options);
  body.addEventListener(
    "click",
    (event) => {
      const button = (event.target as Element).closest<HTMLButtonElement>("button[data-day]");
      const day = button ? toDay(button.dataset.day!) : null;
      if (day !== null) select(day);
    },
    options,
  );
  body.addEventListener(
    "keydown",
    (event) => {
      const weekday = (new Date(focus * dayMs).getUTCDay() - props.weekStart + 7) % 7;
      let target: Day;
      switch (event.key) {
        case "ArrowLeft": target = focus - 1; break;
        case "ArrowRight": target = focus + 1; break;
        case "ArrowUp": target = focus - 7; break;
        case "ArrowDown": target = focus + 7; break;
        case "Home": target = focus - weekday; break;
        case "End": target = focus + 6 - weekday; break;
        case "PageUp": target = addMonths(focus, event.shiftKey ? -12 : -1); break;
        case "PageDown": target = addMonths(focus, event.shiftKey ? 12 : 1); break;
        default: return;
      }
      event.preventDefault();
      focus = target;
      render(true);
    },
    options,
  );
  // A reset of the form, or code of the page, can change the input.
  input.addEventListener(
    "change",
    () => {
      const day = toDay(input.value);
      if (day === selected) return;
      selected = day;
      if (day !== null) focus = day;
      render(false);
    },
    options,
  );
  // The focus goes to the grid when the popover of a date picker opens.
  popover?.addEventListener(
    "toggle",
    (event) => {
      if ((event as ToggleEvent).newState === "open") render(true);
    },
    options,
  );
  render(false);
  show();

  return () => {
    input.className = hadClass;
    input.removeAttribute("tabindex");
    root.replaceChildren();
    root.className = "";
  };
};

export default mount;
