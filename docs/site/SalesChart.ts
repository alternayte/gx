import Chart from "chart.js/auto";
import type { Mount, Props, Update } from "./SalesChart.props";

// The parts of a Chart.js chart that this island uses. A pinned package has
// the type any.
type BarChart = {
  data: { labels: string[]; datasets: { data: number[] }[] };
  update: () => void;
  destroy: () => void;
};

// One chart for each mounted island.
const charts = new WeakMap<HTMLElement, BarChart>();

const labels = (data: Props["data"]): string[] => data.map((point) => point.label);
const values = (data: Props["data"]): number[] => data.map((point) => point.value);

const mount: Mount = (el, { title, data }) => {
  // Chart.js sizes the canvas from its parent, so the parent has a height.
  el.style.position = "relative";
  el.style.height = "16rem";
  const canvas = el.appendChild(document.createElement("canvas"));
  canvas.setAttribute("role", "img");
  canvas.setAttribute("aria-label", title);
  // No animation for a reader who asks for reduced motion.
  const still = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const chart: BarChart = new Chart(canvas, {
    type: "bar",
    data: {
      labels: labels(data),
      datasets: [{ label: title, data: values(data), backgroundColor: "#2563eb", borderRadius: 4 }],
    },
    options: {
      animation: still ? false : { duration: 600 },
      maintainAspectRatio: false,
      color: "#808080",
      plugins: { legend: { display: false } },
      scales: {
        x: { grid: { display: false }, ticks: { color: "#808080" } },
        y: { beginAtZero: true, ticks: { color: "#808080" }, grid: { color: "rgba(128, 128, 128, 0.25)" } },
      },
    },
  });
  charts.set(el, chart);
  // The cleanup runs when the island leaves the page.
  return () => {
    chart.destroy();
    charts.delete(el);
    el.replaceChildren();
  };
};

export default mount;

// New props arrive here. The chart keeps its canvas and moves each bar to
// its new value.
export const update: Update = ({ data }, el) => {
  const chart = charts.get(el);
  if (!chart) return;
  chart.data.labels = labels(data);
  chart.data.datasets[0].data = values(data);
  chart.update();
};
