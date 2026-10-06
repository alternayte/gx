import type { Mount, Props, Update } from "./ChartCanvas.props";

const ns = "http://www.w3.org/2000/svg";

const svg = <K extends keyof SVGElementTagNameMap>(tag: K, attrs: Record<string, string | number>, parent?: Element): SVGElementTagNameMap[K] => {
  const node = document.createElementNS(ns, tag);
  for (const [name, value] of Object.entries(attrs)) node.setAttribute(name, String(value));
  parent?.appendChild(node);
  return node;
};

// niceMax returns a round number at or above the largest value, and the
// step of four ticks below it.
const niceMax = (max: number): { top: number; step: number } => {
  if (max <= 0) return { top: 1, step: 0.25 };
  const rough = max / 4;
  const power = 10 ** Math.floor(Math.log10(rough));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * power).find((s) => s >= rough) ?? 10 * power;
  return { top: step * 4, step };
};

// The size of the drawing in its own units. The SVG scales to the width of
// its box.
const width = 480;
const height = 240;
const pad = { top: 12, right: 12, bottom: 28, left: 44 };

// State is what one mounted chart keeps between a mount and an update.
type State = { props: Props; active: number; draw: () => void };
const states = new WeakMap<HTMLElement, State>();

// The island draws a bar, line or area chart as SVG from the data of the
// props. The server renders the same data as a table; the island takes the
// table out of view and keeps it for a screen reader. With no script the
// page shows the table.
const mount: Mount = (root, props, ctx) => {
  const table = document.getElementById(props.table);
  const c = props.classes;
  const hadClass = table?.className ?? "";
  if (table) table.className = c.hidden;

  root.className = c.root;
  const figure = root.appendChild(document.createElement("div"));
  figure.className = c.figure;
  figure.tabIndex = 0;
  figure.setAttribute("role", "img");
  const drawing = svg("svg", { viewBox: `0 0 ${width} ${height}`, class: c.svg, "aria-hidden": "true" }, figure);
  const tooltip = root.appendChild(document.createElement("div"));
  tooltip.className = c.tooltip;
  tooltip.hidden = true;
  tooltip.setAttribute("role", "status");
  const legend = root.appendChild(document.createElement("ul"));
  legend.className = c.legend;

  const state: State = { props, active: -1, draw: () => {} };
  states.set(root, state);

  const color = (i: number): string => `var(--chart-${(i % 5) + 1})`;
  const plot = { w: width - pad.left - pad.right, h: height - pad.top - pad.bottom };

  state.draw = (): void => {
    const { categories, series, kind } = state.props;
    const numbers = new Intl.NumberFormat(document.documentElement.lang || undefined);
    figure.setAttribute("aria-label", `${state.props.label}. ${series.map((s) => s.name).join(", ")}. ${categories.length} points.`);
    drawing.replaceChildren();
    const largest = Math.max(0, ...series.flatMap((s) => s.values));
    const { top, step } = niceMax(largest);
    const y = (value: number): number => pad.top + plot.h - (value / top) * plot.h;
    const band = categories.length > 0 ? plot.w / categories.length : plot.w;
    const center = (i: number): number => pad.left + band * (i + 0.5);

    for (let tick = 0; tick <= top + step / 2; tick += step) {
      svg("line", { x1: pad.left, x2: width - pad.right, y1: y(tick), y2: y(tick), class: c.grid }, drawing);
      svg("text", { x: pad.left - 6, y: y(tick), "text-anchor": "end", "dominant-baseline": "middle", class: c.axis }, drawing).textContent = numbers.format(tick);
    }
    categories.forEach((name, i) => {
      svg("text", { x: center(i), y: height - 8, "text-anchor": "middle", class: c.axis }, drawing).textContent = name;
    });
    if (state.active >= 0 && state.active < categories.length) {
      svg("rect", { x: pad.left + band * state.active, y: pad.top, width: band, height: plot.h, class: c.highlight }, drawing);
    }

    series.forEach((s, si) => {
      const fill = color(si);
      if (kind === "bar") {
        const inner = band * 0.7;
        const w = inner / series.length;
        s.values.forEach((value, i) => {
          const x = pad.left + band * i + (band - inner) / 2 + w * si;
          const bar = svg("rect", { x: x + 1, y: y(Math.max(value, 0)), width: Math.max(w - 2, 1), height: Math.max(pad.top + plot.h - y(Math.max(value, 0)), 0), rx: 2 }, drawing);
          bar.style.fill = fill;
          bar.setAttribute("data-series", s.name);
          bar.setAttribute("data-point", String(i));
        });
        return;
      }
      const points = s.values.map((value, i) => `${center(i)},${y(Math.max(value, 0))}`);
      if (kind === "area" && points.length > 0) {
        const area = svg("polygon", { points: [`${center(0)},${y(0)}`, ...points, `${center(points.length - 1)},${y(0)}`].join(" ") }, drawing);
        area.style.fill = fill;
        area.style.opacity = "0.25";
      }
      const line = svg("polyline", { points: points.join(" "), fill: "none", "stroke-width": 2, "stroke-linejoin": "round", "stroke-linecap": "round" }, drawing);
      line.style.stroke = fill;
      line.setAttribute("data-series", s.name);
      s.values.forEach((value, i) => {
        const dot = svg("circle", { cx: center(i), cy: y(Math.max(value, 0)), r: i === state.active ? 4 : 2.5 }, drawing);
        dot.style.fill = fill;
        dot.setAttribute("data-series", s.name);
        dot.setAttribute("data-point", String(i));
      });
    });

    legend.replaceChildren();
    series.forEach((s, si) => {
      const item = legend.appendChild(document.createElement("li"));
      item.className = c.legendItem;
      const swatch = item.appendChild(document.createElement("span"));
      swatch.className = c.swatch;
      swatch.style.background = color(si);
      item.appendChild(document.createTextNode(s.name));
    });

    if (state.active >= 0 && state.active < categories.length) {
      tooltip.hidden = false;
      tooltip.replaceChildren();
      tooltip.appendChild(document.createElement("strong")).textContent = categories[state.active];
      for (const s of series) {
        const row = tooltip.appendChild(document.createElement("div"));
        row.textContent = `${s.name}: ${numbers.format(s.values[state.active] ?? 0)}`;
      }
      // The tooltip follows the active point across the width of the box.
      tooltip.style.left = `${(center(state.active) / width) * 100}%`;
    } else {
      tooltip.hidden = true;
    }
  };

  const setActive = (index: number): void => {
    const count = state.props.categories.length;
    const next = index < 0 || count === 0 ? -1 : Math.min(index, count - 1);
    if (next === state.active) return;
    state.active = next;
    state.draw();
  };

  const listen = { signal: ctx.abort };
  figure.addEventListener(
    "pointermove",
    (event) => {
      const box = drawing.getBoundingClientRect();
      const x = ((event.clientX - box.left) / box.width) * width;
      const count = state.props.categories.length;
      if (x < pad.left || x > width - pad.right || count === 0) return setActive(-1);
      setActive(Math.floor(((x - pad.left) / plot.w) * count));
    },
    listen,
  );
  figure.addEventListener("pointerleave", () => setActive(-1), listen);
  figure.addEventListener("blur", () => setActive(-1), listen);
  figure.addEventListener(
    "keydown",
    (event) => {
      const last = state.props.categories.length - 1;
      switch (event.key) {
        case "ArrowRight":
          setActive(Math.min(state.active + 1, last));
          break;
        case "ArrowLeft":
          setActive(state.active <= 0 ? 0 : state.active - 1);
          break;
        case "Home":
          setActive(0);
          break;
        case "End":
          setActive(last);
          break;
        case "Escape":
          setActive(-1);
          break;
        default:
          return;
      }
      event.preventDefault();
    },
    listen,
  );
  state.draw();

  return () => {
    states.delete(root);
    if (table) table.className = hadClass;
    root.replaceChildren();
    root.className = "";
  };
};

export default mount;

// New data from the server draws again. The active point stays.
export const update: Update = (props, root) => {
  const state = states.get(root);
  if (!state) return;
  state.props = props;
  state.draw();
};
