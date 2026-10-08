import type { Mount } from "./NoteCount.props";

// The island counts the characters of the note. It follows a signal of the
// composer, so it shows the count while the user types.
const mount: Mount = (el, { note, limit }, ctx) => {
  const text = el.appendChild(document.createElement("span"));
  return ctx.signal(note).subscribe((value) => {
    const count = [...value].length;
    text.textContent = `${count} of ${limit}`;
    el.toggleAttribute("data-over", count > limit);
  });
};

export default mount;
