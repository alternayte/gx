package round7_test

import "testing"

// TestREQ_ACT_18_TwoRequestsFail checks REQ-ACT-18: "The runtime saves the
// value of each signal that the statements write. It puts the values back
// when the action answers with an error or a rule failure, or when the
// request fails."
//
// The user clicks two times with the network off. The first click saves
// the count 0 and shows 1. The second click saves the count 1, which is the
// optimistic value of the first click, and shows 2. The first request fails
// and the runtime puts 0 back. The second request fails and the runtime
// puts 1 back. No action ran, and the page shows the count 1.
//
// The script uses keep, take and watch of runtime/js/optimistic.ts as
// installCSRF of runtime/js/csrf.ts uses them: take at the start of each
// fetch, and watch around its promise.
func TestREQ_ACT_18_TwoRequestsFail(t *testing.T) {
	got := runBun(t, `
import { keep, take, watch } from "%JS%/optimistic.ts"

let count = 0
const restore = (kept) => { count = kept.stars.count }
// click is one click on <button on:click={Star{}} optimistic:click={$Count++}>:
// the capture handler gives the value to keep and runs the statement, and
// the fetch of the action takes the saved value.
const click = () => {
  keep([{ stars: { count } }], () => {})
  count++
  let fail
  const request = watch(new Promise((_, reject) => { fail = reject }), take(), restore)
  return { request, fail }
}
const first = click()
const second = click()
if (count !== 2) throw new Error("the fixture is wrong: the count is " + count)
first.fail(new Error("offline"))
await first.request.catch(() => {})
second.fail(new Error("offline"))
await second.request.catch(() => {})
console.log(count)
`)
	if got != "0" {
		t.Errorf("after two clicks whose requests fail, the count is %s, want 0: the value before the first click", got)
	}
}
