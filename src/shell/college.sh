#!/bin/bash

# On-demand macOS management component collector.
# Bash 3.2 compatible. The script collects file metadata only after a user
# starts it, asks for confirmation, uploads once, and prints a report URL.

set +e
umask 077

# External configuration.
RUN_MODE="${RUN_MODE:-}"
mdm_lang="${mdm_lang:-}"
COLLEGE_SERVER_URL="${COLLEGE_SERVER_URL:-https://mdm.xrsec.fun}"
COLLEGE_SERVER_URL="${COLLEGE_SERVER_URL%/}"
COLLEGE_OPEN_RESULT="${COLLEGE_OPEN_RESULT:-0}"

# Server paths and network limits.
COLLEGE_SCRIPT_PATH="/college.sh"
COLLEGE_SESSION_PATH="/api/college/session"
COLLEGE_REPORT_PATH="/api/college"
CURL_RETRY_COUNT=2
CURL_CONNECT_TIMEOUT=10
CURL_SESSION_MAX_TIME=30
CURL_UPLOAD_MAX_TIME=60

# Collection and upload limits.
MAX_ITEMS=4500
MAX_PAYLOAD_BYTES=1048576

# Temporary file templates.
ITEMS_FILE_TEMPLATE="/tmp/mdm-college-items-$$.XXXXXX"
PAYLOAD_FILE_TEMPLATE="/tmp/mdm-college-payload-$$.XXXXXX"
RESPONSE_FILE_TEMPLATE="/tmp/mdm-college-response-$$.XXXXXX"
PROCESS_FILE_TEMPLATE="/tmp/mdm-college-processes-$$.XXXXXX"
HOSTS_FILE_TEMPLATE="/tmp/mdm-college-hosts-$$.XXXXXX"

# Global runtime state shared between functions.
TARGET_ROOT=""
PAYLOAD_FILE=""
ITEMS_FILE=""
RESPONSE_FILE=""
PROCESS_FILE=""
HOSTS_FILE=""
PASSWORD_INPUT=""
password=""
REPORT_ID=""
REPORT_URL=""
ITEM_COUNT=0
FIRST_ITEM=1
CURSOR_HIDDEN=0

# Terminal presentation state. Empty values keep redirected output plain.
COLLEGE_RESET=""
COLLEGE_BOLD=""
COLLEGE_MUTED=""
COLLEGE_ACCENT=""
COLLEGE_SAFE=""
COLLEGE_WARNING=""

cleanup() {
  if [ "$CURSOR_HIDDEN" = "1" ] && command_exists tput; then
    tput cnorm 2>/dev/null || true
  fi
  CURSOR_HIDDEN=0
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

setup_terminal_styles() {
  [ -t 1 ] || return 0
  [ "${TERM:-dumb}" != "dumb" ] || return 0
  [ -z "${NO_COLOR:-}" ] || return 0
  COLLEGE_RESET=$(printf '\033[0m')
  COLLEGE_BOLD=$(printf '\033[1m')
  COLLEGE_MUTED=$(printf '\033[2m')
  COLLEGE_ACCENT=$(printf '\033[38;5;37m')
  COLLEGE_SAFE=$(printf '\033[38;5;71m')
  COLLEGE_WARNING=$(printf '\033[38;5;173m')
}

clear_language_menu() {
  local lines="$1"
  while [ "$lines" -gt 0 ]; do
    tput cuu1 2>/dev/null || break
    tput el 2>/dev/null || true
    lines=$((lines - 1))
  done
}

print_language_options() {
  local selected="$1"
  if [ "$selected" -eq 0 ]; then
    printf '  %b▸ 简体中文 ◂%b\n' "$COLLEGE_SAFE$COLLEGE_BOLD" "$COLLEGE_RESET" >&2
    printf '    English\n' >&2
  else
    printf '    简体中文\n' >&2
    printf '  %b▸ English ◂%b\n' "$COLLEGE_SAFE$COLLEGE_BOLD" "$COLLEGE_RESET" >&2
  fi
}

select_language_fallback() {
  local answer=""
  printf '\nChoose language / 选择语言\n' >&2
  printf '  1) 简体中文\n  2) English\n' >&2
  while :; do
    printf 'Select / 请选择 [1-2]: ' >&2
    IFS= read -r answer || return 1
    case "$answer" in
      1) mdm_lang=1; return 0 ;;
      2) mdm_lang=0; return 0 ;;
      *) printf 'Invalid selection / 选择无效。\n' >&2 ;;
    esac
  done
}

select_language() {
  local selected=0
  local key=""
  local sequence=""
  case "$mdm_lang" in
    0|1) export mdm_lang; return 0 ;;
  esac
  if [ ! -t 0 ] || [ ! -t 2 ] || ! command_exists tput || ! tput cols >/dev/null 2>&1; then
    select_language_fallback || return 1
    export mdm_lang
    return 0
  fi

  printf '\nChoose language / 选择语言\n' >&2
  printf '  ↑/↓ Select / 选择    Enter Confirm / 确认\n\n' >&2
  print_language_options "$selected"
  if tput civis 2>/dev/null; then CURSOR_HIDDEN=1; fi
  while :; do
    key=""
    if ! IFS= read -r -s -n 1 key; then
      [ "$CURSOR_HIDDEN" = "1" ] && tput cnorm 2>/dev/null
      CURSOR_HIDDEN=0
      clear_language_menu 5
      return 1
    fi
    if [ -z "$key" ]; then
      [ "$CURSOR_HIDDEN" = "1" ] && tput cnorm 2>/dev/null
      CURSOR_HIDDEN=0
      clear_language_menu 5
      if [ "$selected" -eq 0 ]; then mdm_lang=1; else mdm_lang=0; fi
      export mdm_lang
      return 0
    fi
    if [ "$key" = $'\033' ]; then
      sequence=""
      IFS= read -r -s -n 2 sequence || continue
      case "$sequence" in
        '[A'|'[B') if [ "$selected" -eq 0 ]; then selected=1; else selected=0; fi ;;
        *) continue ;;
      esac
      clear_language_menu 2
      print_language_options "$selected"
    fi
  done
}

college_text() {
  local key="$1"
  if [ "$mdm_lang" = "1" ]; then
    case "$key" in
      RECOVERY_ONLY) printf '%s' '监管分析仅支持正常桌面 macOS，不支持 Recovery。' ;;
      INVALID_RUN_MODE) printf '%s' 'RUN_MODE 必须为 normal。' ;;
      SUDO_REQUIRED) printf '%s' '读取完整系统元数据需要 sudo。' ;;
      PASSWORD_PROMPT) printf '%s' '请输入当前用户密码：' ;;
      PASSWORD_INVALID) printf '%s' '密码不能为空，也不能包含空白字符。' ;;
      PASSWORD_FAILED) printf '%s' '密码验证失败' ;;
      CURL_REQUIRED) printf '%s' '创建报告需要 curl。' ;;
      SESSION_FAILED) printf '%s' '无法创建报告，请检查 HTTPS、网络连接和系统时间。' ;;
      SESSION_HTML) printf '%s' '服务端返回了 HTML，而不是报告会话。' ;;
      INVALID_REPORT_ID) printf '%s' '服务端返回了无效的报告 ID。' ;;
      REPORT_URL_MISSING) printf '%s' '服务端响应中没有报告链接。' ;;
      PAYLOAD_TOO_LARGE) printf '%s' '元数据超过 1 MiB 上传限制，未上传任何内容。' ;;
      UPLOAD_FAILED) printf '%s' '上传失败，请检查 HTTPS、网络连接和系统时间。' ;;
      UPLOAD_HTML) printf '%s' '服务端返回了 HTML，而不是报告响应。' ;;
      UPLOAD_REJECTED) printf '%s' '服务端未接受分析数据。' ;;
      *) printf '%s' "$key" ;;
    esac
  else
    case "$key" in
      RECOVERY_ONLY) printf '%s' 'Management analysis requires normal desktop macOS and is unavailable in Recovery.' ;;
      INVALID_RUN_MODE) printf '%s' 'RUN_MODE must be normal.' ;;
      SUDO_REQUIRED) printf '%s' 'sudo is required to read all system metadata.' ;;
      PASSWORD_PROMPT) printf '%s' 'Enter the current user password: ' ;;
      PASSWORD_INVALID) printf '%s' 'Password cannot be empty or contain whitespace.' ;;
      PASSWORD_FAILED) printf '%s' 'Password verification failed' ;;
      CURL_REQUIRED) printf '%s' 'curl is required to create the report.' ;;
      SESSION_FAILED) printf '%s' 'Could not create the report. Check HTTPS, network access, and system time.' ;;
      SESSION_HTML) printf '%s' 'Server returned HTML instead of a report session.' ;;
      INVALID_REPORT_ID) printf '%s' 'Server returned an invalid report ID.' ;;
      REPORT_URL_MISSING) printf '%s' 'Server response did not contain a report URL.' ;;
      PAYLOAD_TOO_LARGE) printf '%s' 'The metadata exceeds the 1 MiB upload limit; nothing was uploaded.' ;;
      UPLOAD_FAILED) printf '%s' 'Upload failed. Check HTTPS, network access, and system time.' ;;
      UPLOAD_HTML) printf '%s' 'Server returned HTML instead of a report response.' ;;
      UPLOAD_REJECTED) printf '%s' 'Server did not accept the analysis payload.' ;;
      *) printf '%s' "$key" ;;
    esac
  fi
}

print_collection_notice() {
  if [ "$mdm_lang" = "1" ]; then
    printf '\n%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '╭─ MDM 监管分析 ─────────────────────────────────────╮' "$COLLEGE_RESET"
    printf '%b  %s%b\n' "$COLLEGE_BOLD" '只读扫描 · 一次上传 · 不会自动修改系统' "$COLLEGE_RESET"
    printf '%b  %s%b\n' "$COLLEGE_MUTED" '请先确认下面的数据范围，再决定是否继续。' "$COLLEGE_RESET"
    printf '%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '├─ 会采集' "$COLLEGE_RESET"
    printf '  %b●%b %s\n' "$COLLEGE_SAFE" "$COLLEGE_RESET" 'MDM 注册状态与 ADE/DEP 标记'
    printf '  %b●%b %s\n' "$COLLEGE_SAFE" "$COLLEGE_RESET" '系统级管理组件的名称、路径和签名元数据'
    printf '  %b●%b %s\n' "$COLLEGE_SAFE" "$COLLEGE_RESET" '运行中可执行文件的名称或系统路径'
    printf '  %b●%b %s\n' "$COLLEGE_SAFE" "$COLLEGE_RESET" '/etc/hosts 中被覆盖的 Apple 域名'
    printf '%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '├─ 不会采集' "$COLLEGE_RESET"
    printf '  %b●%b %s\n' "$COLLEGE_MUTED" "$COLLEGE_RESET" '密码、序列号、设备名称或任何文件内容'
    printf '  %b●%b %s\n' "$COLLEGE_MUTED" "$COLLEGE_RESET" '进程参数、用户目录路径或临时目录路径'
    printf '  %b●%b %s\n' "$COLLEGE_MUTED" "$COLLEGE_RESET" 'Hosts IP、注释以及非 Apple 域名记录'
    printf '%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '├─ 隐私处理' "$COLLEGE_RESET"
    printf '  %s\n' '云配置 URL 仅保留主机名；结果元数据只上传一次。'
    printf '%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '╰─ 上传目标' "$COLLEGE_RESET"
  else
    printf '\n%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '╭─ MDM MANAGEMENT ANALYSIS ──────────────────────────╮' "$COLLEGE_RESET"
    printf '%b  %s%b\n' "$COLLEGE_BOLD" 'Read-only scan · One upload · No automatic changes' "$COLLEGE_RESET"
    printf '%b  %s%b\n' "$COLLEGE_MUTED" 'Review the data scope below before continuing.' "$COLLEGE_RESET"
    printf '%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '├─ COLLECTED' "$COLLEGE_RESET"
    printf '  %b●%b %s\n' "$COLLEGE_SAFE" "$COLLEGE_RESET" 'MDM enrollment status and ADE/DEP markers'
    printf '  %b●%b %s\n' "$COLLEGE_SAFE" "$COLLEGE_RESET" 'Names, paths, and signatures of system management components'
    printf '  %b●%b %s\n' "$COLLEGE_SAFE" "$COLLEGE_RESET" 'Names or system paths of running executables'
    printf '  %b●%b %s\n' "$COLLEGE_SAFE" "$COLLEGE_RESET" 'Apple domains overridden in /etc/hosts'
    printf '%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '├─ NOT COLLECTED' "$COLLEGE_RESET"
    printf '  %b●%b %s\n' "$COLLEGE_MUTED" "$COLLEGE_RESET" 'Passwords, serial number, device name, or file contents'
    printf '  %b●%b %s\n' "$COLLEGE_MUTED" "$COLLEGE_RESET" 'Process arguments, user-directory paths, or temporary paths'
    printf '  %b●%b %s\n' "$COLLEGE_MUTED" "$COLLEGE_RESET" 'Hosts IPs, comments, or non-Apple domain records'
    printf '%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '├─ PRIVACY' "$COLLEGE_RESET"
    printf '  %s\n' 'Cloud configuration URLs are reduced to hostnames; metadata is uploaded once.'
    printf '%b%s%b\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" '╰─ UPLOAD DESTINATION' "$COLLEGE_RESET"
  fi
  printf '  %b%s%b\n\n' "$COLLEGE_BOLD" "$COLLEGE_SERVER_URL" "$COLLEGE_RESET"
}

print_stage() {
  local text="$2"
  [ "$mdm_lang" = "1" ] || text="$3"
  printf '\n%b[%s]%b %s\n' "$COLLEGE_ACCENT$COLLEGE_BOLD" "$1" "$COLLEGE_RESET" "$text"
}

print_report_result() {
  if [ "$mdm_lang" = "1" ]; then
    printf '\n%b%s%b\n' "$COLLEGE_SAFE$COLLEGE_BOLD" '╭─ 分析完成 ─────────────────────────────────────────╮' "$COLLEGE_RESET"
    printf '  %s\n' '元数据已上传，报告可以立即查看。'
    printf '%b%s%b\n' "$COLLEGE_SAFE$COLLEGE_BOLD" '╰─ 报告链接' "$COLLEGE_RESET"
  else
    printf '\n%b%s%b\n' "$COLLEGE_SAFE$COLLEGE_BOLD" '╭─ ANALYSIS COMPLETE ────────────────────────────────╮' "$COLLEGE_RESET"
    printf '  %s\n' 'Metadata uploaded. The report is ready to view.'
    printf '%b%s%b\n' "$COLLEGE_SAFE$COLLEGE_BOLD" '╰─ REPORT URL' "$COLLEGE_RESET"
  fi
  printf '  %b%s%b\n' "$COLLEGE_BOLD" "$REPORT_URL" "$COLLEGE_RESET"
  if [ "$mdm_lang" = "1" ]; then
    printf '  %b%s%b\n\n' "$COLLEGE_MUTED" '请妥善保管：任何获得完整链接的人都能在有效期内查看报告。' "$COLLEGE_RESET"
  else
    printf '  %b%s%b\n\n' "$COLLEGE_MUTED" 'Keep this URL private: anyone with it can view the report until it expires.' "$COLLEGE_RESET"
  fi
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
      printf '%s\n' "$(college_text RECOVERY_ONLY)" >&2
      exit 1
      ;;
    *) printf '%s\n' "$(college_text INVALID_RUN_MODE)" >&2; exit 1 ;;
  esac

  if [ -d /System/Installation ] || [ -d "/System/Library/CoreServices/Recovery Springboard.app" ]; then
    printf '%s\n' "$(college_text RECOVERY_ONLY)" >&2
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
    printf '%s\n' "$(college_text RECOVERY_ONLY)" >&2
    exit 1
  fi
}

ensure_root() {
  local attempt=1
  local command_status=1
  [ "$(id -u 2>/dev/null)" = "0" ] && return 0
  if ! command_exists sudo; then
    printf '%s\n' "$(college_text SUDO_REQUIRED)" >&2
    exit 1
  fi

  while [ "$attempt" -le 3 ]; do
    read_password_with_feedback "$(college_text PASSWORD_PROMPT)" || PASSWORD_INPUT=""
    password="$PASSWORD_INPUT"
    PASSWORD_INPUT=""
    if ! password_input_is_valid "$password"; then
      password=""
      printf '%s\n' "$(college_text PASSWORD_INVALID)" >&2
    elif printf '%s\n' "$password" | sudo -S -p '' -v >/dev/null 2>&1; then
      break
    else
      password=""
      printf '%s (%s/3).\n' "$(college_text PASSWORD_FAILED)" "$attempt" >&2
    fi
    attempt=$((attempt + 1))
  done
  if [ -z "$password" ] || [ "$attempt" -gt 3 ]; then
    password=""
    exit 1
  fi

  COLLEGE_PASSWORD_STDIN=1
  export RUN_MODE mdm_lang COLLEGE_SERVER_URL COLLEGE_OPEN_RESULT COLLEGE_SCRIPT_PATH COLLEGE_PASSWORD_STDIN
  printf '%s\n' "$password" | sudo -nE /bin/bash -c '/bin/bash <(curl -kfsSL "${COLLEGE_SERVER_URL%/}${COLLEGE_SCRIPT_PATH}")'
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
  PROCESS_FILE=$(mktemp "$PROCESS_FILE_TEMPLATE") || return 0
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
  HOSTS_FILE=$(mktemp "$HOSTS_FILE_TEMPLATE") || return 0
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
    printf '%s\n' "$(college_text CURL_REQUIRED)" >&2
    return 1
  }
  curl -fsSL --retry "$CURL_RETRY_COUNT" --connect-timeout "$CURL_CONNECT_TIMEOUT" --max-time "$CURL_SESSION_MAX_TIME" \
    -X POST "$COLLEGE_SERVER_URL$COLLEGE_SESSION_PATH" \
    -o "$RESPONSE_FILE" || {
      printf '%s\n' "$(college_text SESSION_FAILED)" >&2
      return 1
    }
  if grep -Eiq '<!doctype|<html' "$RESPONSE_FILE" 2>/dev/null; then
    printf '%s\n' "$(college_text SESSION_HTML)" >&2
    return 1
  fi
  REPORT_ID=$(sed -n 's/.*"id":"\([a-f0-9]*\)".*/\1/p' "$RESPONSE_FILE" | head -1)
  REPORT_URL=$(sed -n 's/.*"result_url":"\([^"]*\)".*/\1/p' "$RESPONSE_FILE" | head -1)
  case "$REPORT_ID" in
    ????????????????????????????????) ;;
    *) printf '%s\n' "$(college_text INVALID_REPORT_ID)" >&2; return 1 ;;
  esac
  [ -n "$REPORT_URL" ] || {
    printf '%s\n' "$(college_text REPORT_URL_MISSING)" >&2
    return 1
  }
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
  print_collection_notice
  if [ "$mdm_lang" = "1" ]; then
    printf '%b%s%b' "$COLLEGE_WARNING$COLLEGE_BOLD" '确认扫描并上传？请输入 YES（其他输入均取消）：' "$COLLEGE_RESET"
  else
    printf '%b%s%b' "$COLLEGE_WARNING$COLLEGE_BOLD" 'Scan and upload now? Type YES to continue (anything else cancels): ' "$COLLEGE_RESET"
  fi
  IFS= read -r answer || return 1
  [ "$answer" = "YES" ]
}

validate_payload_size() {
  local bytes=""
  bytes=$(wc -c < "$PAYLOAD_FILE" | tr -d ' ')
  if [ "$mdm_lang" = "1" ]; then
    printf '      已采集 %s 项元数据，共 %s 字节。\n' "$ITEM_COUNT" "$bytes"
  else
    printf '      Collected %s metadata items (%s bytes).\n' "$ITEM_COUNT" "$bytes"
  fi
  if [ "$bytes" -gt "$MAX_PAYLOAD_BYTES" ] 2>/dev/null; then
    printf '%s\n' "$(college_text PAYLOAD_TOO_LARGE)" >&2
    return 1
  fi
  return 0
}

upload_payload() {
  curl -fsSL --retry "$CURL_RETRY_COUNT" --connect-timeout "$CURL_CONNECT_TIMEOUT" --max-time "$CURL_UPLOAD_MAX_TIME" \
    -H 'Content-Type: application/json' \
    --data-binary "@$PAYLOAD_FILE" \
    "$COLLEGE_SERVER_URL$COLLEGE_REPORT_PATH/$REPORT_ID/upload" \
    -o "$RESPONSE_FILE" || {
      printf '%s\n' "$(college_text UPLOAD_FAILED)" >&2
      return 1
    }
  if grep -Eiq '<!doctype|<html' "$RESPONSE_FILE" 2>/dev/null; then
    printf '%s\n' "$(college_text UPLOAD_HTML)" >&2
    return 1
  fi
  if ! grep -q '"ok":true' "$RESPONSE_FILE" 2>/dev/null; then
    printf '%s\n' "$(college_text UPLOAD_REJECTED)" >&2
    return 1
  fi
  print_report_result
  if [ "$COLLEGE_OPEN_RESULT" = "1" ] && [ "$RUN_MODE" = "normal" ] && command_exists open; then
    open "$REPORT_URL" >/dev/null 2>&1 || true
  fi
}

main() {
  read_inherited_password
  setup_terminal_styles
  select_language || exit 1
  detect_environment
  ensure_root "$@"
  select_target_root
  confirm_collection || {
    if [ "$mdm_lang" = "1" ]; then
      printf '\n%b%s%b\n' "$COLLEGE_MUTED" '已取消：未扫描系统，也未上传任何数据。' "$COLLEGE_RESET"
    else
      printf '\n%b%s%b\n' "$COLLEGE_MUTED" 'Cancelled: the system was not scanned and no data was uploaded.' "$COLLEGE_RESET"
    fi
    exit 1
  }
  ITEMS_FILE=$(mktemp "$ITEMS_FILE_TEMPLATE") || exit 1
  PAYLOAD_FILE=$(mktemp "$PAYLOAD_FILE_TEMPLATE") || exit 1
  RESPONSE_FILE=$(mktemp "$RESPONSE_FILE_TEMPLATE") || exit 1
  print_stage '1/3' '正在创建私密报告…' 'Creating a private report…'
  create_report_session || exit 1
  print_stage '2/3' '正在只读扫描系统元数据…' 'Scanning system metadata read-only…'
  collect_metadata
  build_payload
  validate_payload_size || exit 1
  print_stage '3/3' '正在上传分析元数据…' 'Uploading analysis metadata…'
  upload_payload
}

main "$@"
