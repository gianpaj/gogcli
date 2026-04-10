#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Generate a JSON mapping from existing Google Slides text to template placeholders.

Usage:
  gogcli/scripts/templateize-slide.sh PRESENTATION_ID SLIDE_ID [--mode cards|generic]

Examples:
  gogcli/scripts/templateize-slide.sh 11WKpLDQmri3PRYiYZZtFjJAMqjLnFbBP0JvFqGFKXNM g370260aabdc_0_159
  gogcli/scripts/templateize-slide.sh 11WKpLDQmri3PRYiYZZtFjJAMqjLnFbBP0JvFqGFKXNM g370260aabdc_0_159 --mode generic

Notes:
  - Requires ./bin/gog to exist and be authenticated.
  - "cards" mode assumes:
      text 1 = {{title}}
      text 2 = {{subtitle}}
      remaining texts are paired as {{item_N_title}} / {{item_N_body}}
  - "generic" mode uses {{title}}, {{subtitle}}, then slugified placeholders.
EOF
  exit 1
}

if [ "${#}" -lt 2 ]; then
  usage
fi

presentation_id="$1"
slide_id="$2"
shift 2

mode="cards"

while [ "${#}" -gt 0 ]; do
  case "$1" in
    --mode)
      [ "${#}" -ge 2 ] || usage
      mode="$2"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      ;;
  esac
done

case "$mode" in
  cards|generic)
    ;;
  *)
    echo "invalid mode: $mode (expected cards or generic)" >&2
    exit 1
    ;;
esac

if [ ! -x ./bin/gog ]; then
  echo "error: ./bin/gog not found or not executable" >&2
  exit 1
fi

json="$(./bin/gog slides read-slide "$presentation_id" "$slide_id" --recursive --json)"

python3 - "$mode" "$json" <<'PY'
import json
import re
import sys

mode = sys.argv[1]
data = json.loads(sys.argv[2])

texts = []
for item in data.get("textElements", []):
    text = item.get("text", "").strip()
    if text:
        texts.append(text)

if not texts:
    print(json.dumps({}, indent=2, ensure_ascii=False))
    sys.exit(0)

def slugify(text: str) -> str:
    text = text.lower().strip()
    text = re.sub(r"[^a-z0-9]+", "_", text)
    text = re.sub(r"_+", "_", text).strip("_")
    return text or "text"

mapping = {}

if mode == "cards":
    if len(texts) >= 1:
        mapping[texts[0]] = "{{title}}"
    if len(texts) >= 2:
        mapping[texts[1]] = "{{subtitle}}"

    rest = texts[2:]
    for i in range(0, len(rest), 2):
        n = i // 2 + 1
        if i < len(rest):
            mapping[rest[i]] = "{{item_%d_title}}" % n
        if i + 1 < len(rest):
            mapping[rest[i + 1]] = "{{item_%d_body}}" % n
else:
    seen = {}

    for i, text in enumerate(texts):
        if i == 0:
            key = "title"
        elif i == 1:
            key = "subtitle"
        else:
            key = slugify(text)
            if len(key) > 32:
                key = key[:32].rstrip("_")
            if key in seen:
                seen[key] += 1
                key = f"{key}_{seen[key]}"
            else:
                seen[key] = 1

        mapping[text] = "{{" + key + "}}"

print(json.dumps(mapping, indent=2, ensure_ascii=False))
PY
