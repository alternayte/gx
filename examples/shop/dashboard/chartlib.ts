// The drawing code that the chart islands share. The bundler puts it in one
// chunk, so a page with both charts loads it once.
//
// d3-scale is a pinned package: `gx pin d3-scale@4.0.2` stored its build in
// js/vendor/, and the import resolves there with no node.
import { scaleLinear } from "d3-scale";

export type Bar = { label: string; value: number };

// The longest bar is 200 px wide.
const barWidth = 200;

export function draw(el: HTMLElement, kind: string, bars: Bar[]): void {
  const width = scaleLinear()
    .domain([0, Math.max(...bars.map((bar) => bar.value))])
    .range([0, barWidth]);
  const list = el.querySelector<HTMLElement>("[data-bars]") ?? el.appendChild(document.createElement("div"));
  list.setAttribute("data-bars", kind);
  list.replaceChildren(
    ...bars.map((bar) => {
      const row = document.createElement("div");
      row.setAttribute("data-bar", bar.label);
      row.style.width = `${width(bar.value)}px`;
      row.textContent = `${bar.label} ${bar.value}`;
      return row;
    }),
  );
}
