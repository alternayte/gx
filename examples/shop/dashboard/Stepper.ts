import type { Mount } from "./Stepper.props";

// The stepper reads and writes a signal of the page.
const mount: Mount = (el, { qty }, ctx) => {
  const signal = ctx.signal(qty);
  const button = el.appendChild(document.createElement("button"));
  button.onclick = () => signal.set(signal.get() + 1);
  const stop = signal.subscribe((value) => {
    button.textContent = `Quantity ${value}, add one`;
  });
  return stop;
};

export default mount;
