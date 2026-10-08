// The Gx tabs module of a page (REQ-REG-07). The code is in
// tabs-core.ts; this file runs it for the document.
import { install } from './tabs-core'

if (typeof document !== 'undefined') install(document)
