#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Export placeholder mappings for every slide in a Google Slides presentation.

This script reads all slide IDs from ./bin/gog, then reads each slide recursively
and emits one JSON file containing an array of slide objects.

Usage:
  gogcli/scripts/templateize-presentation.sh PRESENTATION_ID [--output FILE] [--mode cards|generic]

Examples:
  gogcli/scripts/templateize-presentation.sh 11WKpLDQmri3PRYiYZZtFjJAMqjLnFbBP0JvFqGFKXNM
  gogcli/scripts/templateize-presentation.sh 11WKpLDQmri3PRYiYZZtFjJAMqjLnFbBP0JvFqGFKXNM --output template-map.json
  gogcli/scripts/templateize-presentation.sh 11WKpLDQmri3PRYiYZZtFjJAMqjLnFbBP0JvFqGFKXNM --mode generic

Output shape:
  [
    {
      "slideNumber": 1,
      "slideObjectId": "p",
      "textElements": [...],
      "mapping": {
        "Original text": "{{title}}"
      }
    }
  ]

Notes:
  - Requires ./bin/gog to exist and be authenticated.
  - Uses `slides read-slide --recursive --json` for each slide.
  - "cards" mode assumes:
      text 1 = {{title}}
      text 2 = {{subtitle}}
      remaining texts are paired as {{item_N_title}} / {{item_N_body}}
  - "generic" mode uses {{title}}, {{subtitle}}, then slugified placeholders.
EOF
  exit 1
}

if [ "${#}" -lt 1 ]; then
  usage
fi

presentation_id="$1"
shift

output_file=""
mode="cards"

while [ "${#}" -gt 0 ]; do
  case "$1" in
    --output)
      [ "${#}" -ge 2 ] || usage
      output_file="$2"
      shift 2
      ;;
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

slides_json="$(
  ./bin/gog slides list-slides "$presentation_id" --json
)"

result_json="$(
  python3 - "$presentation_id" "$mode" "$slides_json" <<'PY'
import json
import re
import subprocess
import sys

presentation_id = sys.argv[1]
mode = sys.argv[2]
slides_payload = json.loads(sys.argv[3])

if isinstance(slides_payload, dict):
    slides = slides_payload.get("slides", [])
elif isinstance(slides_payload, list):
    slides = slides_payload
else:
    raise SystemExit("unexpected JSON from slides list-slides")

def slugify(text: str) -> str:
    text = text.lower().strip()
    text = re.sub(r"[^a-z0-9]+", "_", text)
    text = re.sub(r"_+", "_", text).strip("_")
    return text or "text"

def build_mapping(texts, mode_name):
    mapping = {}

    if mode_name == "cards":
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
        return mapping

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

    return mapping

def read_slide(slide_object_id):
    proc = subprocess.run(
        [
            "./bin/gog",
            "slides",
            "read-slide",
            presentation_id,
            slide_object_id,
            "--recursive",
            "--json",
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(proc.stdout)

output = []

for idx, slide in enumerate(slides, start=1):
    if not isinstance(slide, dict):
        continue

    slide_object_id = (
        slide.get("objectId")
        or slide.get("slideObjectId")
        or slide.get("id")
    )
    if not slide_object_id:
        continue

    slide_data = read_slide(slide_object_id)

    text_elements = slide_data.get("textElements", [])
    texts = []
    for item in text_elements:
        if not isinstance(item, dict):
            continue
        text = str(item.get("text", "")).strip()
        if text:
            texts.append(text)

    output.append({
        "slideNumber": slide_data.get("slideNumber", idx),
        "slideObjectId": slide_data.get("slideObjectId", slide_object_id),
        "textElements": text_elements,
        "mapping": build_mapping(texts, mode),
    })

print(json.dumps(output, indent=2, ensure_ascii=False))
PY
)"

if [ -n "$output_file" ]; then
  printf '%s\n' "$result_json" > "$output_file"
else
  printf '%s\n' "$result_json"
fi
