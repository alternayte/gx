#!/usr/bin/env bash
# check: delve-smoke
# born: 2026-10-04
# failure: Delve cannot stop on a breakpoint in a .gx loop or show p and locals
# rule: REQ-TLS-08: debug a generated app and break inside a .gx loop
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if ! command -v dlv >/dev/null 2>&1; then
  echo "delve-smoke: dlv is required for this check" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

go build -o "$work/gx" ./cmd/gx

mkdir -p "$work/app/ui/card"
cat > "$work/app/go.mod" <<EOF
module app

go 1.25.0

require github.com/alternayte/gx v0.0.0

replace github.com/alternayte/gx => $root
EOF
cat > "$work/app/ui/card/Card.gx" <<'EOF'
package card

props {
  Title string
  Items []string
}

<article>
  for _, it := range p.Items {
    <p>{it}</p>
  }
</article>
EOF
cat > "$work/app/main.go" <<'EOF'
package main

import (
	"fmt"

	"app/ui/card"

	"github.com/alternayte/gx"
)

func main() {
	fmt.Print(gx.String(card.Card(card.CardProps{Title: "hi", Items: []string{"a", "b"}})))
}
EOF

"$work/gx" generate "$work/app"
(cd "$work/app" && go mod tidy >/dev/null 2>&1 && go build -gcflags="all=-N -l" -o app .)

# macOS refuses an unsigned debug target under the native backend.
if [ "$(uname -s)" = "Darwin" ]; then
  cat > "$work/ent.plist" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>com.apple.security.get-task-allow</key><true/></dict></plist>
EOF
  codesign -s - -f --entitlements "$work/ent.plist" "$work/app/app" >/dev/null 2>&1
fi

cat > "$work/init.txt" <<'EOF'
break Card.gx:10
continue
print p.Title
print it
locals
exit
EOF

out="$(cd "$work/app" && dlv exec ./app --init="$work/init.txt" --headless=false --allow-non-terminal-interactive=true --check-go-version=false 2>&1)"
printf '%s\n' "$out"

fail=0
for want in 'Card.gx:10' '"hi"' '"a"' 'it = "a"'; do
  if ! printf '%s' "$out" | grep -qF -- "$want"; then
    echo "delve-smoke: output lacks $want" >&2
    fail=1
  fi
done
exit "$fail"
