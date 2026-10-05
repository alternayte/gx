// Loaded before every spec file (bunfig.toml). A hook here is global, so
// this afterAll runs once, after the last file of the process.
import { afterAll } from 'bun:test'
import { closeBrowser } from './harness'

afterAll(closeBrowser)
