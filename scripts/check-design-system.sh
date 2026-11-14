#!/usr/bin/env bash
# Frontend ratchet. Baseline is scripts/design-system-baseline.txt
# ("file:rule" lines); it may only shrink.
#
# Design system (docs/agents/design-system.md「边界检查」): outside
# src/design/, no literal colors, no px corner radii, no other icon or
# component libraries, no transitions of layout (width, height, position,
# margin, padding) or of `all`: they lay the page out again on every frame.
#
# Bindings (docs/agents/frontend.md「代码结构」): only src/lib/ and a
# feature's controller and data files call Go; everything else imports
# types alone from src/mygo.ts.
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
check motion 'transition(-property)?:[^;]*\b(all|width|height|top|left|right|bottom|inset|margin|padding)\b'
check components "from ['\"](@mui/|antd|element-plus|vuetify|@chakra-ui|naive-ui)"

# A file calls Go when it imports anything but types from src/mygo.ts.
while IFS= read -r file; do
  case "$file" in
    src/mygo.ts | src/lib/* | *.test.ts | */controller.ts | *Controller.ts | */data.ts) continue ;;
  esac
  perl -0ne '
    while (/import\s+(type\s+)?(\*\s+as\s+\w+|\w+|\{[^}]*\})\s+from\s+["\x27](?:\.\.?\/)+mygo["\x27]/g) {
      next if $1;
      my $names = $2;
      $hit = 1 unless $names =~ /^\{/;
      for (split /,/, $names =~ tr/{}//dr) { s/^\s+|\s+$//g; $hit = 1 if $_ ne "" && !/^type\s/ }
    }
    print "$ARGV:bindings\n" if $hit;
  ' "$file" >>"$found"
done < <(find src -type f \( -name '*.ts' -o -name '*.tsx' \) | sort)

sort -u -o "$found" "$found"

if [[ "${1:-}" == "--update-baseline" ]]; then
  cp "$found" "$baseline"
  echo "baseline: $(wc -l <"$baseline" | tr -d ' ') entries"
  exit 0
fi

new=$(comm -23 "$found" <(sort -u "$baseline"))
stale=$(comm -13 "$found" <(sort -u "$baseline"))
if [[ -n "$new" ]]; then
  echo "前端边界检查失败,新增违规(color / radius / icons / components:改用 src/design/ 的 token 与组件;motion:只过渡透明度、位移与颜色;bindings:经 controller 或数据钩子调用 Go):"
  echo "$new"
  exit 1
fi
if [[ -n "$stale" ]]; then
  echo "基线里有已修复的条目,请运行 $0 --update-baseline:"
  echo "$stale"
  exit 1
fi
echo "design system: ok"
