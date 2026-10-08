// The Gx toast module of a page (REQ-REG-07). The code is in
// toast-core.ts; this file runs it for the document.
import { install } from './toast-core'

if (typeof document !== 'undefined') install(document)
