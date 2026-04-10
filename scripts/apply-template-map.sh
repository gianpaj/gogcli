#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Copy a Google Slides deck and replace existing text with template placeholders.

This script:
  1. Copies a source presentation using ./bin/gog slides copy
  2. Reads a template-map JSON file (array of slide objects with per-slide "mapping")
  3. Applies per-slide batched replacements using ./bin/gog slides replace-text-batch --page-id
  4. Writes a JSON summary of the copied presentation and replacements

Usage:
  gogcli/scripts/apply-template-map.sh SOURCE_PRESENTATION_ID TEMPLATE_MAP_JSON NEW_TITLE [--output FILE] [--account EMAIL] [--sleep SECONDS]

Examples:
  gogcli/scripts/apply-template-map.sh \
    11WKpLDQmri3PRYiYZZtFjJAMqjLnFbBP0JvFqGFKXNM \
    template-map-cards.json \
    "TWIN Template"

  gogcli/scripts/apply-template-map.sh \
    11WKpLDQmri3PRYiYZZtFjJAMqjLnFbBP0JvFqGFKXNM \
    template-map-cards.json \
    "TWIN Template" \
    --output applied-template.json

  gogcli/scripts/apply-template-map.sh \
    11WKpLDQmri3PRYiYZZtFjJAMqjLnFbBP0JvFqGFKXNM \
    template-map-cards.json \
    "TWIN Template" \
    --sleep 1

Requirements:
  - Run from the repository root so ./bin/gog exists.
  - ./bin/gog must already be authenticated for Slides + Drive access.
  - Python 3 must be available.
  - The template map file should look like:
      [
        {
          "slideNumber": 1,
          "slideObjectId": "abc",
          "mapping": {
            "Original text": "{{placeholder}}"
          }
        }
      ]

Notes:
  - Replacements are applied per slide using gog slides replace-text-batch with --page-id.
  - If the same source string appears on multiple slides, it can map to different placeholders on different slides.
  - If the same source string appears multiple times on the same slide, all occurrences on that slide will be replaced.
  - Empty source strings are ignored.
EOF
  exit 1
}

if [ "${#}" -lt 3 ]; then
  usage
fi

source_presentation_id="$1"
template_map_file="$2"
new_title="$3"
shift 3

output_file=""
account=""
sleep_seconds="0"

while [ "${#}" -gt 0 ]; do
  case "$1" in
    --output)
      [ "${#}" -ge 2 ] || usage
      output_file="$2"
      shift 2
      ;;
    --account)
      [ "${#}" -ge 2 ] || usage
      account="$2"
      shift 2
      ;;
    --sleep)
      [ "${#}" -ge 2 ] || usage
      sleep_seconds="$2"
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

if [ ! -x ./bin/gog ]; then
  echo "error: ./bin/gog not found or not executable" >&2
  exit 1
fi

if [ ! -f "$template_map_file" ]; then
  echo "error: template map file not found: $template_map_file" >&2
  exit 1
fi

copy_cmd=(./bin/gog slides copy "$source_presentation_id" "$new_title" --json)
if [ -n "$account" ]; then
  copy_cmd=(./bin/gog --account "$account" slides copy "$source_presentation_id" "$new_title" --json)
fi

case "$sleep_seconds" in
  ''|*[!0-9.]*|*.*.*)
    echo "error: --sleep must be a non-negative number" >&2
    exit 1
    ;;
esac

copy_json="$("${copy_cmd[@]}")"

result_json="$(
python3 - "$template_map_file" "$copy_json" "$account" "$source_presentation_id" "$sleep_seconds" <<'PY'
import json
import os
import subprocess
import sys
import tempfile
import time

template_map_path = sys.argv[1]
copy_payload = json.loads(sys.argv[2])
account = sys.argv[3]
source_presentation_id = sys.argv[4]
sleep_seconds = float(sys.argv[5])

with open(template_map_path, "r", encoding="utf-8") as f:
    template_map = json.load(f)

if not isinstance(template_map, list):
    raise SystemExit("template map must be a JSON array")

new_presentation_id = (
    copy_payload.get("id")
    or copy_payload.get("presentationId")
    or copy_payload.get("file", {}).get("id")
)
new_presentation_name = (
    copy_payload.get("name")
    or copy_payload.get("file", {}).get("name")
)
new_presentation_link = (
    copy_payload.get("link")
    or copy_payload.get("webViewLink")
    or copy_payload.get("file", {}).get("webViewLink")
    or f"https://docs.google.com/presentation/d/{new_presentation_id}/edit"
)

if not new_presentation_id:
    raise SystemExit("could not determine copied presentation ID from gog slides copy output")

gog_path = os.path.abspath("./bin/gog")

sources_by_slide = []
replacement_stats = {}
replacement_count = 0

for slide in template_map:
    if not isinstance(slide, dict):
        continue

    slide_number = slide.get("slideNumber")
    slide_object_id = slide.get("slideObjectId")
    mapping = slide.get("mapping", {})

    if not slide_object_id or not isinstance(mapping, dict):
        continue

    slide_sources = []
    replacements = {}

    for raw_source, raw_target in mapping.items():
        source = str(raw_source).strip()
        target = str(raw_target).strip()

        if not source or not target:
            continue

        replacements[source] = target
        slide_sources.append({
            "source": source,
            "placeholder": target,
        })

    if not replacements:
        sources_by_slide.append({
            "slideNumber": slide_number,
            "slideObjectId": slide_object_id,
            "sources": slide_sources,
        })
        continue

    with tempfile.NamedTemporaryFile("w", encoding="utf-8", suffix=".json", delete=False) as tmp:
        json.dump(replacements, tmp, ensure_ascii=False, indent=2)
        tmp.write("\n")
        replacements_path = tmp.name

    try:
        cmd = [gog_path]
        if account:
            cmd.extend(["--account", account])
        cmd.extend([
            "slides",
            "replace-text-batch",
            new_presentation_id,
            "--page-id",
            slide_object_id,
            "--match-case",
            "--replacements",
            replacements_path,
            "--json",
        ])

        proc = subprocess.run(cmd, capture_output=True, text=True)
        if proc.returncode != 0:
            if proc.stderr:
                print(proc.stderr, file=sys.stderr, end="")
            if proc.stdout:
                print(proc.stdout, file=sys.stderr, end="")
            raise SystemExit(
                f"gog slides replace-text-batch failed for slide {slide_object_id}"
            )

        try:
            replace_result = json.loads(proc.stdout)
        except json.JSONDecodeError as e:
            raise SystemExit(
                f"failed to parse gog slides replace-text-batch JSON output for slide {slide_object_id}: {e}"
            )
    finally:
        try:
            os.unlink(replacements_path)
        except OSError:
            pass

    replacements_result = replace_result.get("replacements", {})
    if not isinstance(replacements_result, dict):
        replacements_result = {}

    for source, target in replacements.items():
        entry = replacements_result.get(source, {})
        occurrences = 0
        if isinstance(entry, dict):
            occurrences = int(entry.get("replacements", 0) or 0)

        replacement_stats.setdefault(slide_object_id, []).append({
            "slideNumber": slide_number,
            "source": source,
            "placeholder": target,
            "occurrencesChanged": occurrences,
        })
        replacement_count += 1

    if sleep_seconds > 0:
        time.sleep(sleep_seconds)

    sources_by_slide.append({
        "slideNumber": slide_number,
        "slideObjectId": slide_object_id,
        "sources": slide_sources,
    })

if replacement_count == 0:
    raise SystemExit("no valid replacements found in template map")

result = {
    "sourcePresentationId": source_presentation_id,
    "templatePresentationId": new_presentation_id,
    "name": new_presentation_name,
    "link": new_presentation_link,
    "replacementCount": replacement_count,
    "replacementsBySlide": replacement_stats,
    "slides": sources_by_slide,
}

print(json.dumps(result, indent=2, ensure_ascii=False))
PY
)"

if [ -n "$output_file" ]; then
  printf '%s\n' "$result_json" > "$output_file"
else
  printf '%s\n' "$result_json"
fi
