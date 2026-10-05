package registry

import "strings"

// Merge merges a local edit and an upstream edit of one file against their
// base (REQ-REG-03). It returns the merged text and whether a conflict
// wrote markers.
//
// The merge is line-based. A region that only one side changed takes that
// side. A region both sides changed takes the change when the two texts are
// equal and writes conflict markers otherwise.
func Merge(base, local, upstream string) (string, bool) {
	if local == base {
		return upstream, false
	}
	if upstream == base || local == upstream {
		return local, false
	}
	baseLines := strings.Split(base, "\n")
	localLines := strings.Split(local, "\n")
	upLines := strings.Split(upstream, "\n")
	localHunks := lineHunks(baseLines, localLines)
	upHunks := lineHunks(baseLines, upLines)
	merged, conflict := mergeLines(baseLines, localLines, upLines, localHunks, upHunks)
	return strings.Join(merged, "\n"), conflict
}

// match is one aligned line pair.
type match struct{ a, b int }

// hunk replaces base[baseStart:baseEnd] with other[otherStart:otherEnd]. An
// empty base range is an insertion; an empty other range is a deletion.
type hunk struct {
	baseStart, baseEnd   int
	otherStart, otherEnd int
}

// lineHunks returns the changed regions of other against base.
func lineHunks(base, other []string) []hunk {
	matches := lcsMatches(base, other)
	var out []hunk
	bi, oi := 0, 0
	for _, m := range matches {
		if m.a > bi || m.b > oi {
			out = append(out, hunk{bi, m.a, oi, m.b})
		}
		bi, oi = m.a+1, m.b+1
	}
	if bi < len(base) || oi < len(other) {
		out = append(out, hunk{bi, len(base), oi, len(other)})
	}
	return out
}

// maxLCSCells caps the dynamic programming table. Larger files fall back to
// one whole-file region, which still merges the one-sided cases and reports
// anything else as a conflict.
const maxLCSCells = 4_000_000

// lcsMatches aligns the lines of a and b by their longest common
// subsequence.
func lcsMatches(a, b []string) []match {
	n, m := len(a), len(b)
	if n == 0 || m == 0 || (n+1)*(m+1) > maxLCSCells {
		return nil
	}
	width := m + 1
	dp := make([]int32, (n+1)*width)
	at := func(i, j int) int { return i*width + j }
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				dp[at(i, j)] = dp[at(i+1, j+1)] + 1
			case dp[at(i+1, j)] >= dp[at(i, j+1)]:
				dp[at(i, j)] = dp[at(i+1, j)]
			default:
				dp[at(i, j)] = dp[at(i, j+1)]
			}
		}
	}
	var out []match
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, match{i, j})
			i++
			j++
		case dp[at(i+1, j)] >= dp[at(i, j+1)]:
			i++
		default:
			j++
		}
	}
	return out
}

// mergeLines walks the base lines and applies the hunks of both sides.
func mergeLines(base, local, up []string, localHunks, upHunks []hunk) ([]string, bool) {
	var out []string
	conflict := false
	li, ui := 0, 0
	for p := 0; p <= len(base); {
		l := hunkAt(localHunks, li, p)
		u := hunkAt(upHunks, ui, p)
		if l == nil && u == nil {
			if p == len(base) {
				return out, conflict
			}
			out = append(out, base[p])
			p++
			continue
		}
		// One side or both sides change a region that starts here. Grow
		// the region until no further hunk on either side overlaps it. A
		// hunk of the other side can start inside the region
		// (REQ-REG-03).
		startL, startU := li, ui
		end := p
		if l != nil {
			end = l.baseEnd
			li++
		}
		if u != nil {
			if u.baseEnd > end {
				end = u.baseEnd
			}
			ui++
		}
		for {
			grew := false
			for li < len(localHunks) && localHunks[li].baseStart < end {
				if localHunks[li].baseEnd > end {
					end = localHunks[li].baseEnd
				}
				li++
				grew = true
			}
			for ui < len(upHunks) && upHunks[ui].baseStart < end {
				if upHunks[ui].baseEnd > end {
					end = upHunks[ui].baseEnd
				}
				ui++
				grew = true
			}
			if !grew {
				break
			}
		}
		localText := regionSide(base, local, localHunks, p, end, startL, li)
		upText := regionSide(base, up, upHunks, p, end, startU, ui)
		switch {
		case ui == startU:
			out = append(out, localText...)
		case li == startL:
			out = append(out, upText...)
		case strings.Join(localText, "\n") == strings.Join(upText, "\n"):
			out = append(out, localText...)
		default:
			conflict = true
			out = append(out, "<<<<<<< local")
			out = append(out, localText...)
			out = append(out, "=======")
			out = append(out, upText...)
			out = append(out, ">>>>>>> upstream")
		}
		p = end
	}
	return out, conflict
}

// hunkAt returns the hunk that starts at p, or nil.
func hunkAt(hunks []hunk, i, p int) *hunk {
	if i < len(hunks) && hunks[i].baseStart == p {
		return &hunks[i]
	}
	return nil
}

// regionSide renders one side of a region: the replacement lines of the
// hunks start to stop of that side, and the base lines elsewhere.
func regionSide(base, other []string, hunks []hunk, from, to, start, stop int) []string {
	var out []string
	i := start
	p := from
	for p < to || (i < stop && hunks[i].baseStart == p) {
		if i < stop && hunks[i].baseStart == p {
			h := hunks[i]
			out = append(out, other[h.otherStart:h.otherEnd]...)
			p = h.baseEnd
			i++
			continue
		}
		out = append(out, base[p])
		p++
	}
	return out
}
