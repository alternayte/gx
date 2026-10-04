// The browser entry of the dev-only shadcn reference. It renders the
// reference body of one item so Radix portals and open overlays exist for the
// parity captures (REQ-REG-08).
import { createRoot } from "react-dom/client"
import { useEffect } from "react"
import { refs } from "./refs"

declare global {
  interface Window {
    __REF__?: { name: string; open: boolean }
  }
}

function Ready() {
  useEffect(() => {
    document.documentElement.setAttribute("data-ready", "1")
  }, [])
  return null
}

const info = window.__REF__
const root = document.getElementById("parity-root")
if (info && root) {
  const ref = refs[info.name]
  if (!ref) throw new Error(`refs: unknown item ${info.name}`)
  const body = typeof ref.body === "function" ? ref.body(info.open) : ref.body
  createRoot(root).render(
    <>
      {body}
      <Ready />
    </>,
  )
}
