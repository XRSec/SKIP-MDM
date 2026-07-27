#!/bin/bash

# On-demand macOS management component collector.
# Bash 3.2 compatible. The script collects file metadata only after a user
# starts it, asks for confirmation, uploads once, and prints a report URL.

set +e
umask 077

RUN_MODE="${RUN_MODE:-}"
COLLEGE_SERVER_URL="${COLLEGE_SERVER_URL:-https://mdm.xrsec.fun}"
COLLEGE_SERVER_URL="${COLLEGE_SERVER_URL%/}"
COLLEGE_OPEN_RESULT="${COLLEGE_OPEN_RESULT:-0}"
MAX_ITEMS=4500

TARGET_ROOT=""
PAYLOAD_FILE=""
ITEMS_FILE=""
RESPONSE_FILE=""
PROCESS_FILE=""
HOSTS_FILE=""
PASSWORD_INPUT=""
password=""
REPORT_ID=""
REPORT_PASSWORD=""
REPORT_URL=""
ITEM_COUNT=0
FIRST_ITEM=1

cleanup() {
  PASSWORD_INPUT=""
  password=""
  [ -n "$PAYLOAD_FILE" ] && [ -e "$PAYLOAD_FILE" ] && rm -f "$PAYLOAD_FILE"
  [ -n "$ITEMS_FILE" ] && [ -e "$ITEMS_FILE" ] && rm -f "$ITEMS_FILE"
  [ -n "$RESPONSE_FILE" ] && [ -e "$RESPONSE_FILE" ] && rm -f "$RESPONSE_FILE"
  [ -n "$PROCESS_FILE" ] && [ -e "$PROCESS_FILE" ] && rm -f "$PROCESS_FILE"
  [ -n "$HOSTS_FILE" ] && [ -e "$HOSTS_FILE" ] && rm -f "$HOSTS_FILE"
}

trap cleanup EXIT HUP INT TERM

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

read_password_with_feedback() {
  local prompt="$1"
  local value=""
  local character=""

  PASSWORD_INPUT=""
  printf '%s' "$prompt" >&2
  if [ ! -t 0 ]; then
    IFS= read -r value || return 1
    PASSWORD_INPUT="$value"
    value=""
    printf '\n' >&2
    return 0
  fi

  while IFS= read -r -s -n 1 character; do
    [ -n "$character" ] || break
    case "$character" in
      $'\177'|$'\b')
        if [ -n "$value" ]; then
          value=${value%?}
          printf '\b \b' >&2
        fi
        ;;
      *)
        value="${value}${character}"
        printf '*' >&2
        ;;
    esac
  done
  printf '\n' >&2
  PASSWORD_INPUT="$value"
  value=""
}

password_input_is_valid() {
  case "$1" in
    ''|*[[:space:]]*) return 1 ;;
    *) return 0 ;;
  esac
}

read_inherited_password() {
  [ "${COLLEGE_PASSWORD_STDIN:-0}" = "1" ] || return 0
  IFS= read -r password || password=""
  unset COLLEGE_PASSWORD_STDIN
  if [ -r /dev/tty ]; then
    exec </dev/tty
  fi
  password=""
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
    normal|'') ;;
    recovery)
      printf 'Management analysis is available only in normal desktop macOS, not Recovery.\n' >&2
      exit 1
      ;;
    *) printf 'RUN_MODE must be normal.\n' >&2; exit 1 ;;
  esac

  if [ -d /System/Installation ] || [ -d "/System/Library/CoreServices/Recovery Springboard.app" ]; then
    printf 'Management analysis requires normal desktop macOS and is unavailable in Recovery.\n' >&2
    exit 1
  fi
  if [ -e /dev/console ] && command_exists stat; then
    console_user=$(stat -f '%Su' /dev/console 2>/dev/null)
  fi
  if [ -d /private/var/db/dslocal ] && [ -n "$console_user" ] && [ "$console_user" != "root" ] && [ "$console_user" != "loginwindow" ]; then
    RUN_MODE="normal"
  elif command_exists open && [ -d "/System/Library/CoreServices/Finder.app" ] && [ -d /Users ]; then
    RUN_MODE="normal"
  else
    printf 'Management analysis requires normal desktop macOS and is unavailable in Recovery.\n' >&2
    exit 1
  fi
}

ensure_root() {
  local attempt=1
  local command_status=1
  [ "$(id -u 2>/dev/null)" = "0" ] && return 0
  if ! command_exists sudo; then
    printf 'sudo is required to read all system metadata.\n' >&2
    exit 1
  fi

  while [ "$attempt" -le 3 ]; do
    read_password_with_feedback 'Enter the current user password: ' || PASSWORD_INPUT=""
    password="$PASSWORD_INPUT"
    PASSWORD_INPUT=""
    if ! password_input_is_valid "$password"; then
      password=""
      printf 'Password cannot be empty or contain whitespace.\n' >&2
    elif printf '%s\n' "$password" | sudo -S -p '' -v >/dev/null 2>&1; then
      break
    else
      password=""
      printf 'Password verification failed (%s/3).\n' "$attempt" >&2
    fi
    attempt=$((attempt + 1))
  done
  if [ -z "$password" ] || [ "$attempt" -gt 3 ]; then
    password=""
    exit 1
  fi

  COLLEGE_PASSWORD_STDIN=1
  export RUN_MODE COLLEGE_SERVER_URL COLLEGE_OPEN_RESULT COLLEGE_PASSWORD_STDIN
  printf '%s\n' "$password" | sudo -nE /bin/bash -c '/bin/bash <(curl -kfsSL "${COLLEGE_SERVER_URL%/}/college.sh")'
  command_status=$?
  unset COLLEGE_PASSWORD_STDIN
  password=""
  exit "$command_status"
}

select_target_root() {
  TARGET_ROOT="/"
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
    if ! value=$(plutil -extract "$key" raw -o - "$plist" 2>/dev/null); then
      value=""
    fi
  fi
  if [ -z "$value" ] && [ -x /usr/libexec/PlistBuddy ]; then
    buddy_key=$(printf '%s' "$key" | tr '.' ':')
    if ! value=$(/usr/libexec/PlistBuddy -c "Print :$buddy_key" "$plist" 2>/dev/null); then
      value=""
    fi
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
  local status="$9"
  local detail="${10}"
  [ "$ITEM_COUNT" -ge "$MAX_ITEMS" ] && return 0
  ITEM_COUNT=$((ITEM_COUNT + 1))
  if [ "$FIRST_ITEM" = "0" ]; then
    printf ',\n' >> "$ITEMS_FILE"
  fi
  FIRST_ITEM=0
  printf '{"type":"%s","path":"%s","label":"%s","program":"%s","bundle_id":"%s","team_id":"%s","signing_id":"%s","package_id":"%s","status":"%s","detail":"%s"}' \
    "$(json_escape "$type")" \
    "$(json_escape "$item_path")" \
    "$(json_escape "$label")" \
    "$(json_escape "$program")" \
    "$(json_escape "$bundle_id")" \
    "$(json_escape "$team_id")" \
    "$(json_escape "$signing_id")" \
    "$(json_escape "$package_id")" \
    "$(json_escape "$status")" \
    "$(json_escape "$detail")" >> "$ITEMS_FILE"
}

scan_enrollment_status() {
  local output=""
  local command_status=0
  local mdm_status="unknown"
  local ade_status="unknown"

  if ! command_exists profiles; then
    emit_item "enrollment_status" "" "profiles_command" "" "" "" "" "" "unavailable" ""
    return 0
  fi

  output=$(LC_ALL=C profiles status -type enrollment 2>/dev/null)
  command_status=$?
  if [ "$command_status" -ne 0 ]; then
    emit_item "enrollment_status" "" "profiles_command" "" "" "" "" "" "error" ""
    return 0
  fi

  if printf '%s\n' "$output" | grep -Eiq 'MDM enrollment:[[:space:]]*Yes'; then
    mdm_status="yes"
    if printf '%s\n' "$output" | grep -Eiq 'MDM enrollment:[[:space:]]*Yes.*User Approved'; then
      mdm_status="user_approved"
    fi
  elif printf '%s\n' "$output" | grep -Eiq 'MDM enrollment:[[:space:]]*No'; then
    mdm_status="no"
  fi

  if printf '%s\n' "$output" | grep -Eiq 'Enrolled via (ADE|DEP):[[:space:]]*Yes'; then
    ade_status="yes"
  elif printf '%s\n' "$output" | grep -Eiq 'Enrolled via (ADE|DEP):[[:space:]]*No'; then
    ade_status="no"
  fi

  emit_item "enrollment_status" "" "mdm_enrollment" "" "" "" "" "" "$mdm_status" ""
  emit_item "enrollment_status" "" "automated_enrollment" "" "" "" "" "" "$ade_status" ""
}

cloud_configuration_domain() {
  local plist="$1"
  local url=""
  local authority=""
  url=$(plist_value "$plist" "CloudConfigProfile.ConfigurationURL")
  case "$url" in
    *://*) authority=${url#*://} ;;
    *) return 0 ;;
  esac
  authority=${authority%%/*}
  authority=${authority%%\?*}
  authority=${authority%%\#*}
  authority=${authority##*@}
  case "$authority" in
    \[*\]:*) authority=${authority%%]:*}; authority="$authority]" ;;
    *:*) authority=${authority%%:*} ;;
  esac
  printf '%s\n' "$authority"
}

scan_enrollment_records() {
  local settings=""
  local marker=""
  local marker_path=""
  local marker_status=""
  local detail=""
  settings=$(path_under_root "$TARGET_ROOT" "var/db/ConfigurationProfiles/Settings")

  for marker in \
    .cloudConfigRecordFound \
    .cloudConfigHasActivationRecord \
    .cloudConfigProfileInstalled \
    .profilesAreInstalled \
    .cloudConfigRecordNotFound \
    .cloudConfigNoActivationRecord \
    .cloudConfigUserSkippedEnrollment; do
    marker_path="$settings/$marker"
    marker_status="absent"
    detail=""
    if [ -e "$marker_path" ]; then
      marker_status="present"
      if [ "$marker" = ".cloudConfigRecordFound" ] && [ -r "$marker_path" ]; then
        detail=$(cloud_configuration_domain "$marker_path")
      fi
    fi
    emit_item "enrollment_record" "$(normalize_path "$marker_path")" "$marker" "" "" "" "" "" "$marker_status" "$detail"
  done
}

scan_running_processes() {
  local process_path=""
  local process_name=""
  command_exists ps || return 0
  PROCESS_FILE=$(mktemp "/tmp/mdm-college-processes-$$.XXXXXX") || return 0
  ps -axo comm= 2>/dev/null | awk 'NF { sub(/^[[:space:]]+/, ""); if (!seen[$0]++) print }' > "$PROCESS_FILE"
  while IFS= read -r process_path; do
    [ -n "$process_path" ] || continue
    case "$process_path" in
      /Users/*|/Volumes/*|/tmp/*|/private/tmp/*|/var/folders/*|/private/var/folders/*) continue ;;
    esac
    process_name=${process_path##*/}
    case "$process_path" in
      /*) emit_item "running_process" "$process_path" "$process_name" "" "" "" "" "" "running" "" ;;
      *) emit_item "running_process" "" "$process_name" "" "" "" "" "" "running" "" ;;
    esac
  done < "$PROCESS_FILE"
  rm -f "$PROCESS_FILE"
  PROCESS_FILE=""
}

scan_apple_hosts_overrides() {
  local hosts_file=""
  local hostname=""
  hosts_file=$(path_under_root "$TARGET_ROOT" "etc/hosts")
  [ -r "$hosts_file" ] || return 0
  HOSTS_FILE=$(mktemp "/tmp/mdm-college-hosts-$$.XXXXXX") || return 0
  awk '
    {
      sub(/#.*/, "")
      for (i = 2; i <= NF; i++) {
        host = tolower($i)
        sub(/\.$/, "", host)
        if (host == "apple.com" || host ~ /\.apple\.com$/) {
          if (!seen[host]++) print host
        }
      }
    }
  ' "$hosts_file" > "$HOSTS_FILE" 2>/dev/null
  while IFS= read -r hostname; do
    [ -n "$hostname" ] || continue
    emit_item "hosts_override" "" "$hostname" "" "" "" "" "" "present" ""
  done < "$HOSTS_FILE"
  rm -f "$HOSTS_FILE"
  HOSTS_FILE=""
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
  scan_enrollment_status
  scan_enrollment_records
  scan_apple_hosts_overrides
  scan_launch_plists "launch_daemon" "$library/LaunchDaemons"
  scan_launch_plists "launch_agent" "$library/LaunchAgents"
  scan_applications
  scan_bundle_directory "system_extension" "$library/SystemExtensions" ".systemextension"
  scan_bundle_directory "kernel_extension" "$library/Extensions" ".kext"
  scan_direct_entries "privileged_helper" "$library/PrivilegedHelperTools"
  scan_direct_entries "application_support" "$library/Application Support"
  scan_direct_entries "preference" "$library/Preferences"
  scan_direct_entries "managed_preference" "$library/Managed Preferences"
  scan_running_processes
  scan_package_receipts
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

confirm_collection() {
  local answer=""
  printf '\nThis report will scan enrollment status, ADE/DEP markers, Apple domains overridden in /etc/hosts, running executable names or paths, and system-level management component metadata.\n'
  printf 'It excludes hosts IP addresses and non-Apple entries, process arguments, user-directory and temporary paths, file contents, serial number, and device hostname. The cloud configuration URL is reduced to its hostname.\n'
  printf 'Scan this Mac and upload the resulting metadata once to %s? Type YES to continue: ' "$COLLEGE_SERVER_URL"
  IFS= read -r answer || return 1
  [ "$answer" = "YES" ]
}

validate_payload_size() {
  local bytes=""
  bytes=$(wc -c < "$PAYLOAD_FILE" | tr -d ' ')
  printf '\nCollected %s metadata items (%s bytes).\n' "$ITEM_COUNT" "$bytes"
  if [ "$bytes" -gt 1048576 ] 2>/dev/null; then
    printf 'The metadata exceeds the 1 MiB upload limit; nothing was uploaded.\n' >&2
    return 1
  fi
  return 0
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
  read_inherited_password
  detect_environment
  ensure_root "$@"
  select_target_root
  confirm_collection || {
    printf 'Scan cancelled.\n'
    exit 1
  }
  ITEMS_FILE=$(mktemp "/tmp/mdm-college-items-$$.XXXXXX") || exit 1
  PAYLOAD_FILE=$(mktemp "/tmp/mdm-college-payload-$$.XXXXXX") || exit 1
  RESPONSE_FILE=$(mktemp "/tmp/mdm-college-response-$$.XXXXXX") || exit 1
  create_report_session || exit 1
  printf 'Scanning target: %s\n' "$TARGET_ROOT"
  collect_metadata
  build_payload
  validate_payload_size || exit 1
  upload_payload
}

main "$@"
