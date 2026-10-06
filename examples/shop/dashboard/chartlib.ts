// The drawing code that the chart islands share. The bundler puts it in one
// chunk, so a page with both charts loads it once.
export type Bar = { label: string; value: number };

export function draw(el: HTMLElement, kind: string, bars: Bar[]): void {
  const list = el.querySelector<HTMLElement>("[data-bars]") ?? el.appendChild(document.createElement("div"));
  list.setAttribute("data-bars", kind);
  list.replaceChildren(
    ...bars.map((bar) => {
      const row = document.createElement("div");
      row.setAttribute("data-bar", bar.label);
      row.style.width = `${bar.value * 2}px`;
      row.textContent = `${bar.label} ${bar.value}`;
      return row;
    }),
  );
}
