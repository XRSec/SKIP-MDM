#!/bin/bash

# On-demand macOS management component collector.
# Bash 3.2 compatible. The script collects file metadata only after a user
# starts it, asks for confirmation, uploads once, and prints a report URL.

set +e
umask 077

RUN_MODE="${RUN_MODE:-}"
TARGET_VOLUME="${TARGET_VOLUME:-}"
COLLEGE_SERVER_URL="${COLLEGE_SERVER_URL:-https://mdm.xrsec.fun}"
COLLEGE_ALLOW_HTTP="${COLLEGE_ALLOW_HTTP:-0}"
COLLEGE_AUTO_CONFIRM="${COLLEGE_AUTO_CONFIRM:-0}"
COLLEGE_OPEN_RESULT="${COLLEGE_OPEN_RESULT:-0}"
COLLEGE_ALLOW_PARTIAL="${COLLEGE_ALLOW_PARTIAL:-0}"
MAX_ITEMS=4500

SCRIPT_PATH="$0"
TARGET_ROOT=""
PAYLOAD_FILE=""
ITEMS_FILE=""
RESPONSE_FILE=""
REPORT_ID=""
REPORT_PASSWORD=""
REPORT_URL=""
ITEM_COUNT=0
FIRST_ITEM=1

cleanup() {
  [ -n "$PAYLOAD_FILE" ] && [ -e "$PAYLOAD_FILE" ] && rm -f "$PAYLOAD_FILE"
  [ -n "$ITEMS_FILE" ] && [ -e "$ITEMS_FILE" ] && rm -f "$ITEMS_FILE"
  [ -n "$RESPONSE_FILE" ] && [ -e "$RESPONSE_FILE" ] && rm -f "$RESPONSE_FILE"
}

trap cleanup EXIT HUP INT TERM

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

path_under_root() {
  local root="$1"
  local relative="${2#/}"
  if [ "$root" = "/" ]; then
    printf '/%s\n' "$relative"
  else
    printf '%s/%s\n' "${root%/}" "$relative"
  fi
}

normalize_path() {
  local value="$1"
  if [ "$TARGET_ROOT" != "/" ]; then
    case "$value" in
      "$TARGET_ROOT"/*) value="/${value#"$TARGET_ROOT"/}" ;;
    esac
  fi
  printf '%s\n' "$value"
}

detect_environment() {
  local console_user=""
  case "$RUN_MODE" in
    normal|recovery) return 0 ;;
    '') ;;
    *) printf 'RUN_MODE must be normal or recovery.\n' >&2; exit 1 ;;
  esac

  if [ -e /dev/console ] && command_exists stat; then
    console_user=$(stat -f '%Su' /dev/console 2>/dev/null)
  fi
  if [ -d /private/var/db/dslocal ] && [ -n "$console_user" ] && [ "$console_user" != "root" ] && [ "$console_user" != "loginwindow" ]; then
    RUN_MODE="normal"
  elif [ -d /Volumes ] && { [ -d /System/Installation ] || [ -d /System/Library/CoreServices ]; }; then
    RUN_MODE="recovery"
  elif command_exists open && [ -d /Users ]; then
    RUN_MODE="normal"
  else
    RUN_MODE="recovery"
  fi
}

ensure_root() {
  [ "$(id -u 2>/dev/null)" = "0" ] && return 0
  if [ "$COLLEGE_ALLOW_PARTIAL" = "1" ]; then
    printf 'Warning: continuing without root; unreadable system metadata will be omitted.\n' >&2
    return 0
  fi
  if [ "$RUN_MODE" = "recovery" ]; then
    printf 'Recovery collection must run as root.\n' >&2
    exit 1
  fi
  if ! command_exists sudo || [ ! -f "$SCRIPT_PATH" ]; then
    printf 'Run this saved script with sudo to read all system metadata.\n' >&2
    exit 1
  fi
  exec sudo env \
    "RUN_MODE=$RUN_MODE" \
    "TARGET_VOLUME=$TARGET_VOLUME" \
    "COLLEGE_SERVER_URL=$COLLEGE_SERVER_URL" \
    "COLLEGE_ALLOW_HTTP=$COLLEGE_ALLOW_HTTP" \
    "COLLEGE_AUTO_CONFIRM=$COLLEGE_AUTO_CONFIRM" \
    "COLLEGE_OPEN_RESULT=$COLLEGE_OPEN_RESULT" \
    "COLLEGE_ALLOW_PARTIAL=$COLLEGE_ALLOW_PARTIAL" \
    /bin/bash "$SCRIPT_PATH" "$@"
  exit 1
}

is_data_root() {
  local root="$1"
  [ -d "$root/Library" ] && [ -d "$root/Applications" ] && [ -d "$root/var/db" ]
}

select_target_root() {
  local candidate=""
  local answer=""
  local index=0
  local selected=0
  local candidates=()

  if [ "$RUN_MODE" = "normal" ]; then
    TARGET_ROOT="/"
    return 0
  fi

  if [ -n "$TARGET_VOLUME" ]; then
    candidate="${TARGET_VOLUME%/}"
    if is_data_root "$candidate"; then
      TARGET_ROOT="$candidate"
      return 0
    fi
    for candidate in "${TARGET_VOLUME%/} - Data" "${TARGET_VOLUME%/} - 数据"; do
      if is_data_root "$candidate"; then
        TARGET_ROOT="$candidate"
        return 0
      fi
    done
    printf 'TARGET_VOLUME is not a mounted macOS Data volume: %s\n' "$TARGET_VOLUME" >&2
    exit 1
  fi

  for candidate in /Volumes/*; do
    [ -d "$candidate" ] || continue
    is_data_root "$candidate" || continue
    candidates[${#candidates[@]}]="$candidate"
  done
  if [ "${#candidates[@]}" -eq 0 ]; then
    printf 'No mounted macOS Data volume was found. Mount it in Disk Utility first.\n' >&2
    exit 1
  fi
  if [ "${#candidates[@]}" -eq 1 ]; then
    TARGET_ROOT="${candidates[0]}"
    return 0
  fi
  printf 'Choose the target macOS volume:\n' >&2
  index=0
  while [ "$index" -lt "${#candidates[@]}" ]; do
    printf '  %s) %s\n' "$((index + 1))" "${candidates[$index]}" >&2
    index=$((index + 1))
  done
  while :; do
    printf 'Selection [1-%s]: ' "${#candidates[@]}" >&2
    IFS= read -r answer || exit 1
    case "$answer" in ''|*[!0-9]*) continue ;; esac
    selected=$((answer - 1))
    if [ "$selected" -ge 0 ] 2>/dev/null && [ "$selected" -lt "${#candidates[@]}" ] 2>/dev/null; then
      TARGET_ROOT="${candidates[$selected]}"
      return 0
    fi
  done
}

json_escape() {
  printf '%s' "$1" | awk '
    BEGIN { ORS="" }
    {
      if (NR > 1) printf "\\n"
      gsub(/\\/, "\\\\")
      gsub(/\"/, "\\\"")
      gsub(/\t/, "\\t")
      gsub(/\r/, "\\r")
      printf "%s", $0
    }
  '
}

plist_value() {
  local plist="$1"
  local key="$2"
  local buddy_key=""
  local value=""
  if command_exists plutil; then
    value=$(plutil -extract "$key" raw -o - "$plist" 2>/dev/null)
  fi
  if [ -z "$value" ] && [ -x /usr/libexec/PlistBuddy ]; then
    buddy_key=$(printf '%s' "$key" | tr '.' ':')
    value=$(/usr/libexec/PlistBuddy -c "Print :$buddy_key" "$plist" 2>/dev/null)
  fi
  printf '%s\n' "$value"
}

signature_value() {
  local target="$1"
  local key="$2"
  command_exists codesign || return 0
  codesign -dvv "$target" 2>&1 | awk -F= -v wanted="$key" '$1 == wanted { sub(/^[^=]*=/, ""); print; exit }'
}

emit_item() {
  local type="$1"
  local item_path="$2"
  local label="$3"
  local program="$4"
  local bundle_id="$5"
  local team_id="$6"
  local signing_id="$7"
  local package_id="$8"
  [ "$ITEM_COUNT" -ge "$MAX_ITEMS" ] && return 0
  ITEM_COUNT=$((ITEM_COUNT + 1))
  if [ "$FIRST_ITEM" = "0" ]; then
    printf ',\n' >> "$ITEMS_FILE"
  fi
  FIRST_ITEM=0
  printf '{"type":"%s","path":"%s","label":"%s","program":"%s","bundle_id":"%s","team_id":"%s","signing_id":"%s","package_id":"%s"}' \
    "$(json_escape "$type")" \
    "$(json_escape "$item_path")" \
    "$(json_escape "$label")" \
    "$(json_escape "$program")" \
    "$(json_escape "$bundle_id")" \
    "$(json_escape "$team_id")" \
    "$(json_escape "$signing_id")" \
    "$(json_escape "$package_id")" >> "$ITEMS_FILE"
}

scan_launch_plists() {
  local type="$1"
  local directory="$2"
  local plist=""
  local label=""
  local program=""
  for plist in "$directory"/*.plist; do
    [ -f "$plist" ] || continue
    label=$(plist_value "$plist" "Label")
    program=$(plist_value "$plist" "Program")
    [ -n "$program" ] || program=$(plist_value "$plist" "ProgramArguments.0")
    emit_item "$type" "$(normalize_path "$plist")" "$label" "$program" "" "" "" ""
  done
}

emit_bundle() {
  local type="$1"
  local bundle="$2"
  local info="$bundle/Contents/Info.plist"
  local bundle_id=""
  local signing_id=""
  local team_id=""
  [ -d "$bundle" ] || return 0
  [ -r "$info" ] && bundle_id=$(plist_value "$info" "CFBundleIdentifier")
  signing_id=$(signature_value "$bundle" "Identifier")
  team_id=$(signature_value "$bundle" "TeamIdentifier")
  emit_item "$type" "$(normalize_path "$bundle")" "" "" "$bundle_id" "$team_id" "$signing_id" ""
}

scan_applications() {
  local applications=""
  local bundle=""
  applications=$(path_under_root "$TARGET_ROOT" "Applications")
  for bundle in "$applications"/*.app "$applications"/*/*.app; do
    [ -d "$bundle" ] || continue
    emit_bundle "application" "$bundle"
  done
}

scan_bundle_directory() {
  local type="$1"
  local directory="$2"
  local suffix="$3"
  local bundle=""
  for bundle in "$directory"/*"$suffix" "$directory"/*/*"$suffix"; do
    [ -d "$bundle" ] || continue
    emit_bundle "$type" "$bundle"
  done
}

scan_direct_entries() {
  local type="$1"
  local directory="$2"
  local entry=""
  local signing_id=""
  local team_id=""
  [ -d "$directory" ] || return 0
  for entry in "$directory"/*; do
    [ -e "$entry" ] || [ -L "$entry" ] || continue
    signing_id=""
    team_id=""
    if [ "$type" = "privileged_helper" ] && [ -f "$entry" ]; then
      signing_id=$(signature_value "$entry" "Identifier")
      team_id=$(signature_value "$entry" "TeamIdentifier")
    fi
    emit_item "$type" "$(normalize_path "$entry")" "" "" "" "$team_id" "$signing_id" ""
  done
}

scan_package_receipts() {
  local receipts=""
  local package_id=""
  [ "$RUN_MODE" = "normal" ] || return 0
  command_exists pkgutil || return 0
  receipts=$(mktemp "/tmp/mdm-college-receipts-$$.XXXXXX") || return 0
  pkgutil --pkgs > "$receipts" 2>/dev/null
  while IFS= read -r package_id; do
    [ -n "$package_id" ] || continue
    emit_item "package_receipt" "" "" "" "" "" "" "$package_id"
  done < "$receipts"
  rm -f "$receipts"
}

collect_metadata() {
  local library=""
  library=$(path_under_root "$TARGET_ROOT" "Library")
  scan_launch_plists "launch_daemon" "$library/LaunchDaemons"
  scan_launch_plists "launch_agent" "$library/LaunchAgents"
  scan_applications
  scan_bundle_directory "system_extension" "$library/SystemExtensions" ".systemextension"
  scan_bundle_directory "kernel_extension" "$library/Extensions" ".kext"
  scan_direct_entries "privileged_helper" "$library/PrivilegedHelperTools"
  scan_direct_entries "application_support" "$library/Application Support"
  scan_direct_entries "preference" "$library/Preferences"
  scan_direct_entries "managed_preference" "$library/Managed Preferences"
  scan_package_receipts
}

validate_server_url() {
  COLLEGE_SERVER_URL="${COLLEGE_SERVER_URL%/}"
  case "$COLLEGE_SERVER_URL" in
    https://*) return 0 ;;
    http://127.0.0.1:*|http://localhost:*) [ "$COLLEGE_ALLOW_HTTP" = "1" ] && return 0 ;;
  esac
  printf 'COLLEGE_SERVER_URL must use HTTPS. HTTP is allowed only for local testing with COLLEGE_ALLOW_HTTP=1.\n' >&2
  exit 1
}

create_report_session() {
  command_exists curl || {
    printf 'curl is required to create the report.\n' >&2
    return 1
  }
  curl -fsSL --retry 2 --connect-timeout 10 --max-time 30 \
    -X POST "$COLLEGE_SERVER_URL/api/college/session" \
    -o "$RESPONSE_FILE" || {
      printf 'Could not create the report. Check HTTPS, network access, and system time.\n' >&2
      return 1
    }
  if grep -Eiq '<!doctype|<html' "$RESPONSE_FILE" 2>/dev/null; then
    printf 'Server returned HTML instead of a report session.\n' >&2
    return 1
  fi
  REPORT_ID=$(sed -n 's/.*"id":"\([a-f0-9]*\)".*/\1/p' "$RESPONSE_FILE" | head -1)
  REPORT_PASSWORD=$(sed -n 's/.*"report_password":"\([0-9]*\)".*/\1/p' "$RESPONSE_FILE" | head -1)
  REPORT_URL=$(sed -n 's/.*"result_url":"\([^"]*\)".*/\1/p' "$RESPONSE_FILE" | head -1)
  case "$REPORT_ID" in
    ????????????????????????????????) ;;
    *) printf 'Server returned an invalid report ID.\n' >&2; return 1 ;;
  esac
  case "$REPORT_PASSWORD" in
    ??????) ;;
    *) printf 'Server returned an invalid report password.\n' >&2; return 1 ;;
  esac
  [ -n "$REPORT_URL" ] || {
    printf 'Server response did not contain a report URL.\n' >&2
    return 1
  }
  printf '\nReport URL: %s\n' "$REPORT_URL"
  printf 'Report password: %s\n\n' "$REPORT_PASSWORD"
}

build_payload() {
  local collected_at=""
  local os_version=""
  local architecture=""
  collected_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null)
  if [ "$RUN_MODE" = "normal" ] && command_exists sw_vers; then
    os_version=$(sw_vers -productVersion 2>/dev/null)
  fi
  architecture=$(uname -m 2>/dev/null)
  case "$architecture" in x86_64|i386) architecture="amd64" ;; arm64) architecture="arm64" ;; esac
  {
    printf '{"schema_version":1,"collected_at":"%s","run_mode":"%s","os_version":"%s","architecture":"%s","items":[\n' \
      "$(json_escape "$collected_at")" "$(json_escape "$RUN_MODE")" "$(json_escape "$os_version")" "$(json_escape "$architecture")"
    cat "$ITEMS_FILE"
    printf '\n]}\n'
  } > "$PAYLOAD_FILE"
}

confirm_upload() {
  local answer=""
  local bytes=""
  bytes=$(wc -c < "$PAYLOAD_FILE" | tr -d ' ')
  printf '\nCollected %s metadata items (%s bytes).\n' "$ITEM_COUNT" "$bytes"
  if [ "$bytes" -gt 1048576 ] 2>/dev/null; then
    printf 'The metadata exceeds the 1 MiB upload limit; nothing was uploaded.\n' >&2
    return 1
  fi
  printf 'Included: system launch plists, apps, extensions, helper tools, top-level support/preferences, and package receipt IDs.\n'
  printf 'Excluded: file contents, process lists, user directories, serial number, hostname, and command arguments after the executable path.\n'
  [ "$COLLEGE_AUTO_CONFIRM" = "1" ] && return 0
  printf 'Upload once to %s for analysis? Type YES to continue: ' "$COLLEGE_SERVER_URL"
  IFS= read -r answer || return 1
  [ "$answer" = "YES" ]
}

upload_payload() {
  curl -fsSL --retry 2 --connect-timeout 10 --max-time 60 \
    -H 'Content-Type: application/json' \
    -H "X-Report-Password: $REPORT_PASSWORD" \
    --data-binary "@$PAYLOAD_FILE" \
    "$COLLEGE_SERVER_URL/api/college/$REPORT_ID/upload" \
    -o "$RESPONSE_FILE" || {
      printf 'Upload failed. Check HTTPS, network access, and system time.\n' >&2
      return 1
    }
  if grep -Eiq '<!doctype|<html' "$RESPONSE_FILE" 2>/dev/null; then
    printf 'Server returned HTML instead of a report response.\n' >&2
    return 1
  fi
  if ! grep -q '"ok":true' "$RESPONSE_FILE" 2>/dev/null; then
    printf 'Server did not accept the analysis payload.\n' >&2
    return 1
  fi
  printf '\nAnalysis uploaded successfully.\n'
  printf 'Report URL: %s\n' "$REPORT_URL"
  printf 'Report password: %s\n' "$REPORT_PASSWORD"
  if [ "$COLLEGE_OPEN_RESULT" = "1" ] && [ "$RUN_MODE" = "normal" ] && command_exists open; then
    open "$REPORT_URL" >/dev/null 2>&1 || true
  fi
}

main() {
  detect_environment
  ensure_root "$@"
  select_target_root
  validate_server_url
  ITEMS_FILE=$(mktemp "/tmp/mdm-college-items-$$.XXXXXX") || exit 1
  PAYLOAD_FILE=$(mktemp "/tmp/mdm-college-payload-$$.XXXXXX") || exit 1
  RESPONSE_FILE=$(mktemp "/tmp/mdm-college-response-$$.XXXXXX") || exit 1
  create_report_session || exit 1
  printf 'Scanning target: %s\n' "$TARGET_ROOT"
  collect_metadata
  build_payload
  confirm_upload || {
    printf 'Upload cancelled.\n'
    exit 1
  }
  upload_payload
}

main "$@"
