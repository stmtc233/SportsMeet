#!/usr/bin/env bash
# Design-system ratchet (docs/agents/design-system.md「边界检查」):
# outside src/design/, no literal colors, no px corner radii, no other
# icon or component libraries. Baseline is scripts/design-system-baseline.txt
# ("file:rule" lines); it may only shrink.
#
#   scripts/check-design-system.sh                    check
#   scripts/check-design-system.sh --update-baseline  rewrite the baseline
set -euo pipefail
cd "$(dirname "$0")/.."

baseline=scripts/design-system-baseline.txt
touch "$baseline"

found=$(mktemp)
trap 'rm -f "$found"' EXIT

check() { # rule, extended regex
  grep -rEln --include='*.ts' --include='*.tsx' --include='*.vue' --include='*.svelte' --include='*.css' \
    --exclude-dir=design "$2" src 2>/dev/null | grep -v '^src/mygo.ts$' | sed "s|$|:$1|" >>"$found" || true
}

check color '#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\('
check radius 'border-radius:[^;]*[0-9]px'
check icons "from ['\"](@mui/icons|react-icons|@heroicons|@tabler/icons|@fortawesome)"
check components "from ['\"](@mui/|antd|element-plus|vuetify|@chakra-ui|naive-ui)"

sort -u -o "$found" "$found"

if [[ "${1:-}" == "--update-baseline" ]]; then
  cp "$found" "$baseline"
  echo "baseline: $(wc -l <"$baseline" | tr -d ' ') entries"
  exit 0
fi

new=$(comm -23 "$found" <(sort -u "$baseline"))
stale=$(comm -13 "$found" <(sort -u "$baseline"))
if [[ -n "$new" ]]; then
  echo "设计系统检查失败,新增违规(改用 src/design/ 的 token 与组件):"
  echo "$new"
  exit 1
fi
if [[ -n "$stale" ]]; then
  echo "基线里有已修复的条目,请运行 $0 --update-baseline:"
  echo "$stale"
  exit 1
fi
echo "design system: ok"
