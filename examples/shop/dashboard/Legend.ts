import type { Mount } from "./Legend.props";

const mount: Mount = (el, { labels }) => {
  el.textContent = `Legend: ${labels.join(", ")}`;
};

export default mount;
