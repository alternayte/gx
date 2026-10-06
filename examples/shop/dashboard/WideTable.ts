import type { Mount } from "./WideTable.props";

const mount: Mount = (el, { data }) => {
  const table = el.appendChild(document.createElement("table"));
  for (const point of data) {
    const row = table.insertRow();
    row.insertCell().textContent = point.label;
    row.insertCell().textContent = String(point.value);
  }
};

export default mount;
