import { draw } from "./chartlib";
import type { Mount, Props, Update } from "./BarChart.props";

// The chart keeps a zoom level that the server does not know. A click
// raises it.
const render = (el: HTMLElement, { round, data }: Props): void => {
  let heading = el.querySelector("h2");
  if (!heading) {
    heading = el.appendChild(document.createElement("h2"));
    const zoom = el.appendChild(document.createElement("button"));
    zoom.setAttribute("data-zoom", "1");
    zoom.textContent = "Zoom 1";
    zoom.onclick = () => {
      const next = Number(zoom.getAttribute("data-zoom")) + 1;
      zoom.setAttribute("data-zoom", String(next));
      zoom.textContent = `Zoom ${next}`;
    };
  }
  heading.textContent = `Revenue, round ${round}`;
  draw(el, "bar", data);
};

const mount: Mount = (el, props) => {
  render(el, props);
  el.setAttribute("data-mounts", String(Number(el.getAttribute("data-mounts") ?? 0) + 1));
};

export default mount;

// New props from the server change the bars. The zoom level stays.
export const update: Update = (props, el) => render(el, props);
