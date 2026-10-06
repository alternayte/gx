import { draw } from "./chartlib";
import type { Mount } from "./Sparkline.props";

// This island has no update export: new props mount it again.
let mounts = 0;

const mount: Mount = (el, { data }) => {
  mounts++;
  draw(el, "spark", data);
  el.setAttribute("data-mounts", String(mounts));
};

export default mount;
