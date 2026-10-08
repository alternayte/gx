// The Gx component behaviour runtime of a page (REQ-REG-07). The code is in
// behavior-core.ts; this file runs it for the document.
import { install } from './behavior-core'

if (typeof document !== 'undefined') install(document)
