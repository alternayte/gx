import type { Mount } from "./InputOTPSlots.props";

// The island shows the value of one native input as a row of slots. The
// input stays the control: it takes the keys, the pointer and a paste, and
// a form sends its value. With no script the page shows the plain input.
const mount: Mount = (el, props, ctx) => {
  const input = document.getElementById(props.input);
  if (!(input instanceof HTMLInputElement)) {
    throw new Error(`no input with the id ${props.input}`);
  }
  const before = input.className;
  input.className = props.overlayClass;
  el.className = "flex items-center";
  el.setAttribute("aria-hidden", "true");

  const slots: HTMLElement[] = [];
  for (let i = 0; i < props.length; i++) {
    if (props.group > 0 && i > 0 && i % props.group === 0) {
      const separator = el.appendChild(document.createElement("div"));
      separator.className = props.separatorClass;
      separator.setAttribute("data-otp-separator", "");
    }
    const slot = el.appendChild(document.createElement("div"));
    slot.className = props.slotClass;
    slot.setAttribute("data-otp-slot", String(i));
    const first = i === 0 || (props.group > 0 && i % props.group === 0);
    const last = i === props.length - 1 || (props.group > 0 && (i + 1) % props.group === 0);
    slot.setAttribute("data-first", String(first));
    slot.setAttribute("data-last", String(last));
    slots.push(slot);
  }
  const active = props.activeClass.split(" ").filter(Boolean);

  const render = (): void => {
    // The code is digits only, also after a paste.
    const digits = input.value.replace(/\D/g, "").slice(0, props.length);
    if (digits !== input.value) input.value = digits;
    const focused = document.activeElement === input;
    // The active slot is the one at the caret, or the last one of a full
    // code.
    const caret = Math.min(input.selectionStart ?? digits.length, props.length - 1);
    slots.forEach((slot, i) => {
      const isActive = focused && i === caret;
      for (const name of active) slot.classList.toggle(name, isActive);
      slot.setAttribute("data-active", String(isActive));
      slot.replaceChildren();
      if (i < digits.length) {
        slot.textContent = digits[i];
      } else if (isActive) {
        slot.appendChild(document.createElement("div")).className = props.caretClass;
      }
    });
  };

  const options = { signal: ctx.abort };
  for (const type of ["input", "focus", "blur", "keyup", "click", "select"]) {
    input.addEventListener(type, render, options);
  }
  // The browser cuts a pasted text at maxlength before this code sees it,
  // so "12-34 56" would lose digits. The island takes the digits of the
  // whole text and puts them at the caret.
  input.addEventListener(
    "paste",
    (event) => {
      const text = event.clipboardData?.getData("text") ?? "";
      event.preventDefault();
      const start = input.selectionStart ?? input.value.length;
      const end = input.selectionEnd ?? start;
      const pasted = text.replace(/\D/g, "");
      const value = (input.value.slice(0, start) + pasted + input.value.slice(end)).slice(0, props.length);
      input.value = value;
      const caret = Math.min(start + pasted.length, value.length);
      input.setSelectionRange(caret, caret);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    },
    options,
  );
  // A reset of the form changes the value with no input event.
  input.form?.addEventListener("reset", () => setTimeout(render), options);
  document.addEventListener("selectionchange", render, options);
  render();

  return () => {
    input.className = before;
    el.replaceChildren();
  };
};

export default mount;
