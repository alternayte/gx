// The Gx overlay module of a page (REQ-REG-07). The code is in
// overlay-core.ts; this file runs it for the document.
import { install } from './overlay-core'

if (typeof document !== 'undefined') install(document)
