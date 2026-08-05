#!/bin/bash

# Standalone MDM maintenance utility for macOS 11.5+ and macOS Recovery.
# Bash 3.2 compatible. No telemetry or client logging is used. English is
# always embedded.

set +e
umask 077

# ---------------------------------------------------------------------------
# User configuration
# ---------------------------------------------------------------------------

# Runtime overrides: RUN_MODE=normal|recovery, DRY_RUN=1, TARGET_VOLUME=...
RUN_MODE="${RUN_MODE:-}"
DRY_RUN="${DRY_RUN:-0}"
TARGET_VOLUME="${TARGET_VOLUME:-}"
mdm_lang="${mdm_lang:-}"
TRASH_DATE_FORMAT="${TRASH_DATE_FORMAT:-%Y%m%d%H%M%S}"

# Web service base URL. Use https://mdm.xrsec.fun in production. The current
# loopback default is for local development only.
#MDM_SERVER_URL="${MDM_SERVER_URL:-http://192.168.5.14:9000}"
MDM_SERVER_URL="${MDM_SERVER_URL:-https://mdm.xrsec.fun}"

# Chinese language pack. Override with MDM_LANG_PACK_URL or, for local tests,
# MDM_LANG_PACK_FILE. Keep production deployments on HTTPS.
LANG_PACK_URL="${MDM_LANG_PACK_URL:-${MDM_SERVER_URL%/}/lang/zh-CN.lang}"

# Entries are lowercase, space-delimited substring matches. Override the
# built-in set with MDM_KEYWORDS, then append deployment-specific terms with
# MDM_EXTRA_KEYWORDS.
DEFAULT_MDM_KEYWORDS="com.apple.mdmclient com.apple.managedclient com.apple.devicemanagement com.apple.devicemanagementclient addigy ivanti jamf kandji mobileiron mosyle rippling airwatch falcon freshservice intune osquery tinyapp us.zoom workspaceone jumpcloud teslad orthus com.ws1 com.vmware.deem fleetdm ninjarmm automox tanium corplink"
MDM_KEYWORDS="${MDM_KEYWORDS:-$DEFAULT_MDM_KEYWORDS}"
MDM_EXTRA_KEYWORDS="${MDM_EXTRA_KEYWORDS:-}"
MDM_KEYWORDS="$MDM_KEYWORDS $MDM_EXTRA_KEYWORDS"

# ---------------------------------------------------------------------------
# UI configuration
# ---------------------------------------------------------------------------

COL_NC='\033[0m'
COL_YELLOW='\033[1;33m'
COL_RED='\033[1;31m'
COL_GREEN='\033[1;32m'
COL_CYAN='\033[1;36m'

# ---------------------------------------------------------------------------
# Internal runtime state -- do not configure these values directly
# ---------------------------------------------------------------------------

TEMP_FILES=""
TARGET_ROOT=""
SYSTEM_ROOT=""
CURRENT_USER=""
LOGIN_USER="${SUDO_USER:-}"
SESSION_PASSWORD=""
PASSWORD_INPUT=""
MDM_PATH=""
LIBRARY_PATH=""
LANG_PACK=""
DEVICE_SERIAL=""
REQUIRED_LANG_KEYS="LANGUAGE_PROMPT SELECT_PROMPT INVALID_OPTION LEGAL_NOTICE_TITLE LEGAL_NOTICE_SCOPE LEGAL_NOTICE_RISK LEGAL_NOTICE_PROHIBITED LEGAL_NOTICE_RESPONSIBILITY LEGAL_NOTICE_NO_WARRANTY LEGAL_NOTICE_NETWORK LEGAL_NOTICE_PROMPT LEGAL_NOTICE_DECLINED ROOT_REQUEST ROOT_ACTIVE SUDO_PASSWORD_PROMPT SUDO_PASSWORD_EMPTY SUDO_PASSWORD_INVALID PASSWORD_VERIFYING PASSWORD_VERIFIED SUDO_UNAVAILABLE REEXEC_FAILED MODE_NORMAL MODE_RECOVERY INVALID_RUN_MODE DISK_NOT_FOUND DISK_LOCKED DISK_ENCRYPTED DISK_UNLOCK_PASSWORD DISK_UNLOCK_FAILED DISK_MOUNT_FAILED CHOOSE_DISK TARGET_INVALID TARGET_SELECTED COMMAND_MISSING TRASH_USER_MISSING TRASH_CREATE_FAILED TRASH_MOVE_FAILED MAIN_MENU BYPASS_MDM BYPASS_START HOSTS_UPDATING TEMP_FAILED HOSTS_PROCESS_FAILED CREATE_USER RESET_PASSWORD DISABLE_SIP ENABLE_SIP CLEAN_HOSTS CLEAN_WIFI CHANGE_ROOT_PASSWORD DISABLE_ROOT EXIT DONE PARTIAL_DONE PROTECTED_HINT FAILED DRY_RUN_NOTICE SERIAL_NUMBER CONTACT_EMAIL CONTACT_WECHAT SERIAL_UNAVAILABLE NO_MDM_DIR HOSTS_MISSING HOSTS_UPDATED PROFILE_CLEANING SERVICE_CLEANING FILEVAULT_CHECKING FILEVAULT_ALREADY_OFF FILEVAULT_SELECT_USER FILEVAULT_PASSWORD FILEVAULT_EMPTY_PASSWORD FILEVAULT_DISABLING FILEVAULT_DISABLED RESTART_HINT RECOVERY_ONLY NORMAL_ONLY USERNAME_PROMPT REALNAME_PROMPT PASSWORD_PROMPT INVALID_USERNAME INVALID_USER_ID USER_EXISTS USER_HOME_EXISTS USER_CREATED USER_CREATE_FAILED USER_AUTH_FAILED AUTO_CREATE_ADMIN SELECT_USER NO_USER PASSWORD_TOOL_MISSING SIP_TOOL_MISSING CONFIRM_DESTRUCTIVE CANCELLED ROOT_PASSWORD_PROMPT ROOT_DISABLED LANGUAGE_FALLBACK"
LEGAL_NOTICE_CONFIRMED=0
LEGAL_NOTICE_PING_HANDLED=0
CURSOR_HIDDEN=0
TRASH_SEQUENCE=0

cleanup() {
  local item=""
  local old_ifs="$IFS"
  SESSION_PASSWORD=""
  PASSWORD_INPUT=""
  restore_cursor
  IFS='|'
  for item in $TEMP_FILES; do
    [ -n "$item" ] && [ -e "$item" ] && rm -f "$item"
  done
  IFS="$old_ifs"
}

on_signal() {
  cleanup
  exit 130
}

trap cleanup EXIT
trap on_signal HUP INT TERM

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

hide_cursor() {
  [ -t 2 ] || return 0
  if command_exists tput && tput civis 2>/dev/null; then
    :
  else
    printf '\033[?25l' >&2
  fi
  CURSOR_HIDDEN=1
}

restore_cursor() {
  [ "$CURSOR_HIDDEN" = "1" ] || return 0
  if command_exists tput && tput cnorm 2>/dev/null; then
    :
  else
    printf '\033[?25h' >&2
  fi
  CURSOR_HIDDEN=0
}

cursor_up_one() {
  if command_exists tput && tput cuu1 2>/dev/null; then
    return 0
  fi
  printf '\033[1A' >&2
}

erase_current_line() {
  if command_exists tput && tput el 2>/dev/null; then
    printf '\r' >&2
    return 0
  fi
  printf '\033[2K\r' >&2
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

t() {
  local key="$1"
  local value=""
  if [ "$mdm_lang" = "1" ] && [ -n "$LANG_PACK" ] && [ -r "$LANG_PACK" ]; then
    value=$(awk -v wanted="$key" 'BEGIN { FS="\t" } $1 == wanted { sub(/^[^\t]*\t/, ""); print; exit }' "$LANG_PACK")
  fi
  if [ -n "$value" ]; then
    printf '%s' "$value"
    return 0
  fi
  case "$key" in
      LANGUAGE_PROMPT) printf '%s' "Choose Language / 选择语言" ;;
      SELECT_PROMPT) printf '%s' "Enter Number" ;;
      INVALID_OPTION) printf '%s' "Invalid, Try Again" ;;
      LEGAL_NOTICE_TITLE) printf '%s' "IMPORTANT: AUTHORIZATION AND RISK NOTICE" ;;
      LEGAL_NOTICE_SCOPE) printf '%s' "For non-commercial security research, learning, and maintenance of devices you own or are expressly authorized to manage only." ;;
      LEGAL_NOTICE_RISK) printf '%s' "This tool modifies system, device-management, network, file, service, and disk-encryption settings. It may reduce security, cause data loss, or make the Mac unusable." ;;
      LEGAL_NOTICE_PROHIBITED) printf '%s' "Do not use it on an unauthorized device, to evade lawful school or organization management, for commercial purposes, or for any unlawful or rights-infringing activity." ;;
      LEGAL_NOTICE_RESPONSIBILITY) printf '%s' "Verify ownership and authorization, comply with applicable law and policy, back up all important data, and accept responsibility for the operation and its consequences." ;;
      LEGAL_NOTICE_NO_WARRANTY) printf '%s' "Provided as-is, without any guarantee of fitness, success, or recovery. To the fullest extent permitted by law, the authors and contributors disclaim liability for loss caused by use or misuse; liability that cannot lawfully be excluded remains unaffected." ;;
      LEGAL_NOTICE_NETWORK) printf '%s' "If you continue, this client will send the device serial number to the configured service, which records all valid request IP addresses in the proxy forwarding chain and the request time and may derive an approximate location from the first IP address, solely to record this acknowledgement. A separately initiated management report has its own upload confirmation and may include enrollment status, ADE/DEP marker presence, the enrollment-service hostname, Apple domains overridden in /etc/hosts, system management metadata, and running executable names or paths, but not Hosts IP addresses, non-Apple Hosts entries, process arguments, or user-directory and temporary-directory paths." ;;
      LEGAL_NOTICE_PROMPT) printf '%s' "Do you consent to the described data transfer and confirm your authorization, understanding of the risks, and acceptance of the non-commercial restriction? [y/N]" ;;
      LEGAL_NOTICE_DECLINED) printf '%s' "Authorization and risk notice was not confirmed; exiting" ;;
      ROOT_REQUEST) printf '%s' "Root Access" ;;
      ROOT_ACTIVE) printf '%s' "Running as Root" ;;
      SUDO_PASSWORD_PROMPT) printf '%s' "Current User Password" ;;
      SUDO_PASSWORD_EMPTY) printf '%s' "No Empty or Spaced Passwords" ;;
      SUDO_PASSWORD_INVALID) printf '%s' "Password must not contain spaces. Default 1234" ;;
      PASSWORD_VERIFYING) printf '%s' "Checking Password" ;;
      PASSWORD_VERIFIED) printf '%s' "Password OK" ;;
      SUDO_UNAVAILABLE) printf '%s' "sudo Not Found; Run as Root" ;;
      REEXEC_FAILED) printf '%s' "sudo Relaunch Failed" ;;
      MODE_NORMAL) printf '%s' "Desktop Mode" ;;
      MODE_RECOVERY) printf '%s' "Recovery Mode" ;;
      INVALID_RUN_MODE) printf '%s' "RUN_MODE: normal or recovery" ;;
      DISK_NOT_FOUND) printf '%s' "No macOS Volume; Check Disk Utility" ;;
      DISK_LOCKED) printf '%s' "Locked" ;;
      DISK_ENCRYPTED) printf '%s' "Disk Encryption On" ;;
      DISK_UNLOCK_PASSWORD) printf '%s' "Admin Password to Unlock Disk" ;;
      DISK_UNLOCK_FAILED) printf '%s' "Data Volume Unlock Failed" ;;
      DISK_MOUNT_FAILED) printf '%s' "Volume Mount Failed" ;;
      CHOOSE_DISK) printf '%s' "Choose System Volume" ;;
      TARGET_INVALID) printf '%s' "Invalid Target Volume; Stopped" ;;
      TARGET_SELECTED) printf '%s' "Target Root" ;;
      COMMAND_MISSING) printf '%s' "Missing Command" ;;
      TRASH_USER_MISSING) printf '%s' "Trash User Not Found" ;;
      TRASH_CREATE_FAILED) printf '%s' "Trash Setup Failed" ;;
      TRASH_MOVE_FAILED) printf '%s' "Move to Trash Failed" ;;
      MAIN_MENU) printf '%s' "Choose an Option" ;;
      BYPASS_MDM) printf '%s' "Clean & Disable MDM" ;;
      BYPASS_START) printf '%s' "Cleaning MDM" ;;
      HOSTS_UPDATING) printf '%s' "Updating Hosts" ;;
      TEMP_FAILED) printf '%s' "Temp File Failed" ;;
      HOSTS_PROCESS_FAILED) printf '%s' "Hosts Update Failed" ;;
      CREATE_USER) printf '%s' "Create Admin User" ;;
      RESET_PASSWORD) printf '%s' "Open Reset Password" ;;
      DISABLE_SIP) printf '%s' "Disable SIP" ;;
      ENABLE_SIP) printf '%s' "Enable SIP" ;;
      CLEAN_HOSTS) printf '%s' "Clean Apple Hosts" ;;
      CLEAN_WIFI) printf '%s' "Clean Wi-Fi/APNS Caches" ;;
      CHANGE_ROOT_PASSWORD) printf '%s' "Set Root Password" ;;
      DISABLE_ROOT) printf '%s' "Disable Root User" ;;
      EXIT) printf '%s' "Exit" ;;
      DONE) printf '%s' "Done" ;;
      PARTIAL_DONE) printf '%s' "Done with Errors" ;;
      PROTECTED_HINT) printf '%s' "Try in macOS Recovery" ;;
      FAILED) printf '%s' "Failed" ;;
      DRY_RUN_NOTICE) printf '%s' "DRY_RUN=1: Preview Only" ;;
      SERIAL_NUMBER) printf '%s' "Serial Number" ;;
      CONTACT_EMAIL) printf '%s' "Email: xrsec@qq.com" ;;
      CONTACT_WECHAT) printf '%s' "WeChat: xr_sec" ;;
      SERIAL_UNAVAILABLE) printf '%s' "Serial Unavailable; Offline Still Works" ;;
      NO_MDM_DIR) printf '%s' "ConfigurationProfiles Missing; Mount Data" ;;
      HOSTS_MISSING) printf '%s' "Hosts File Missing or Unreadable" ;;
      HOSTS_UPDATED) printf '%s' "Hosts Updated" ;;
      PROFILE_CLEANING) printf '%s' "Cleaning MDM Profiles" ;;
      SERVICE_CLEANING) printf '%s' "Cleaning Services & Files" ;;
      FILEVAULT_CHECKING) printf '%s' "Checking FileVault" ;;
      FILEVAULT_ALREADY_OFF) printf '%s' "FileVault Already Off" ;;
      FILEVAULT_SELECT_USER) printf '%s' "Choose FileVault User" ;;
      FILEVAULT_PASSWORD) printf '%s' "Enter FileVault User Password" ;;
      FILEVAULT_EMPTY_PASSWORD) printf '%s' "No Empty or Spaced Passwords" ;;
      FILEVAULT_DISABLING) printf '%s' "Disabling FileVault" ;;
      FILEVAULT_DISABLED) printf '%s' "FileVault Disable Accepted" ;;
      RESTART_HINT) printf '%s' "Restart Mac When Done" ;;
      RECOVERY_ONLY) printf '%s' "Recovery Only" ;;
      NORMAL_ONLY) printf '%s' "Desktop macOS Only" ;;
      USERNAME_PROMPT) printf '%s' "New Username (Blank = Auto)" ;;
      REALNAME_PROMPT) printf '%s' "Display Name (Blank = Apple)" ;;
      PASSWORD_PROMPT) printf '%s' "Enter New Admin Password" ;;
      INVALID_USERNAME)   printf '%s' "Use A-Z, a-z, 0-9, . _ -; Start with a letter or _" ;;
      INVALID_USER_ID) printf '%s' "Use an Unused UID from 501-60000" ;;
      USER_EXISTS) printf '%s' "User Already Exists" ;;
      USER_HOME_EXISTS) printf '%s' "Home Folder Already Exists" ;;
      USER_CREATED) printf '%s' "Admin User Created" ;;
      USER_CREATE_FAILED) printf '%s' "User Creation Failed; Check Volume" ;;
      USER_AUTH_FAILED) printf '%s' "User Authentication Setup Failed" ;;
      AUTO_CREATE_ADMIN) printf '%s' "No Regular User; Create Admin First" ;;
      SELECT_USER) printf '%s' "Choose User" ;;
      NO_USER) printf '%s' "No Regular User" ;;
      PASSWORD_TOOL_MISSING) printf '%s' "resetpassword Not Found" ;;
      SIP_TOOL_MISSING) printf '%s' "csrutil Not Found" ;;
      CONFIRM_DESTRUCTIVE) printf '%s' "Modify Selected System? Type YES" ;;
      CANCELLED) printf '%s' "Cancelled" ;;
      ROOT_PASSWORD_PROMPT) printf '%s' "Enter New Root Password" ;;
      ROOT_DISABLED) printf '%s' "Root User Disabled" ;;
      LANGUAGE_FALLBACK) printf '%s' "Chinese Pack Failed; Using English" ;;
      *) printf '%s' "$key" ;;
  esac
}

validate_language_pack() {
  local file="$1"
  [ -s "$file" ] || return 1
  LC_ALL=C awk -v required="$REQUIRED_LANG_KEYS" '
    BEGIN { FS="\t"; ok=1 }
    /^[[:space:]]*$/ { next }
    {
      if (NF != 2 || $1 !~ /^[A-Z0-9_]+$/ || seen[$1]++) ok=0
    }
    END {
      count=split(required, keys, " ")
      for (i=1; i<=count; i++) if (!seen[keys[i]]) ok=0
      exit(ok ? 0 : 1)
    }
  ' "$file" || return 1
  if LC_ALL=C grep -Eq '<(!DOCTYPE|html|HTML)|^[[:space:]]*[{[]' "$file" 2>/dev/null; then
    return 1
  fi
  return 0
}

load_language_pack() {
  local tmp=""
  [ "$mdm_lang" = "1" ] || return 0
  tmp=$(mktemp "/tmp/mdm-lang-$$.XXXXXX") || {
    mdm_lang=0
    msg_err "$(t LANGUAGE_FALLBACK)"
    return 0
  }
  TEMP_FILES="${TEMP_FILES}|${tmp}"

  if [ -n "${MDM_LANG_PACK_FILE:-}" ] && [ -r "$MDM_LANG_PACK_FILE" ]; then
    cp "$MDM_LANG_PACK_FILE" "$tmp" 2>/dev/null || true
  elif command_exists curl; then
    curl -fSL --retry 2 --connect-timeout 5 "$LANG_PACK_URL" -o "$tmp" 2>/dev/null || true
  fi
  if validate_language_pack "$tmp"; then
    LANG_PACK="$tmp"
    return 0
  fi
  rm -f "$tmp"
  LANG_PACK=""
  mdm_lang=0
  export mdm_lang
  msg_err "$(t LANGUAGE_FALLBACK)"
}

msg_info() {
  printf "${COL_YELLOW}[~]${COL_NC} %s\n" "$1" >&2
}

msg_ok() {
  printf "${COL_GREEN}[✓]${COL_NC} %s\n" "$1" >&2
}

msg_err() {
  printf "${COL_RED}[✗]${COL_NC} %s\n" "$1" >&2
}

msg_debug_cmd() {
  local arg=""
  printf '%b' "${COL_CYAN}[DRY_RUN]${COL_NC}" >&2
  for arg in "$@"; do
    printf " %s" "$arg" >&2
  done
  printf '\n' >&2
}

run_cmd_i() {
  # Invisible command: print in DRY_RUN, otherwise suppress command output.
  if [ "$DRY_RUN" = "1" ]; then
    msg_debug_cmd "$@"
    return 0
  fi
  "$@" >/dev/null 2>&1
}

run_cmd_v() {
  # Visible command: print in DRY_RUN, otherwise inherit the terminal.
  if [ "$DRY_RUN" = "1" ]; then
    msg_debug_cmd "$@"
    return 0
  fi
  "$@"
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
    if [ -z "$character" ]; then
      break
    fi
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
  return 0
}

password_input_is_valid() {
  case "$1" in
    ''|*[[:space:]]*) return 1 ;;
    *) return 0 ;;
  esac
}

choose_number_fallback() {
  local prompt="$1"
  shift
  local count=$#
  local item=""
  local answer=""
  local index=1

  [ "$count" -gt 0 ] || return 1
  printf '\n%s\n' "$prompt" >&2
  for item in "$@"; do
    printf '  %s) %s\n' "$index" "$item" >&2
    index=$((index + 1))
  done
  while :; do
    printf '%s [1-%s]: ' "$(t SELECT_PROMPT)" "$count" >&2
    IFS= read -r answer || return 1
    case "$answer" in
      ''|*[!0-9]*) msg_err "$(t INVALID_OPTION)" ;;
      *)
        if [ "$answer" -ge 1 ] 2>/dev/null && [ "$answer" -le "$count" ] 2>/dev/null; then
          SELECTED_INDEX=$((answer - 1))
          return 0
        fi
        msg_err "$(t INVALID_OPTION)"
        ;;
    esac
  done
}

choose_number() {
  local prompt="$1"
  shift
  local options=("$@")
  local count=${#options[@]}
  local selected=0
  local index=0
  local key=""
  local sequence=""

  if [ "$count" -eq 0 ]; then
    return 1
  fi
  if [ ! -t 0 ] || [ ! -t 2 ]; then
    choose_number_fallback "$prompt" "${options[@]}"
    return $?
  fi

  printf '\n%s\n' "$prompt" >&2
  printf '  ↑/↓ 选择 / Select    Enter 确认 / Confirm\n\n' >&2
  while [ "$index" -lt "$count" ]; do
    if [ "$index" -eq "$selected" ]; then
      printf "  ${COL_GREEN}▸ %s ◂${COL_NC}\n" "${options[$index]}" >&2
    else
      printf '    %s\n' "${options[$index]}" >&2
    fi
    index=$((index + 1))
  done
  hide_cursor

  while :; do
    key=""
    if ! IFS= read -r -s -n 1 key; then
      restore_cursor
      return 1
    fi
    if [ -z "$key" ]; then
      restore_cursor
      SELECTED_INDEX="$selected"
      return 0
    fi
    if [ "$key" = $'\033' ]; then
      sequence=""
      IFS= read -r -s -n 2 sequence || continue
      case "$sequence" in
        '[A')
          selected=$((selected - 1))
          [ "$selected" -lt 0 ] && selected=$((count - 1))
          ;;
        '[B')
          selected=$((selected + 1))
          [ "$selected" -ge "$count" ] && selected=0
          ;;
        *) continue ;;
      esac

      index=0
      while [ "$index" -lt "$count" ]; do
        cursor_up_one
        index=$((index + 1))
      done
      index=0
      while [ "$index" -lt "$count" ]; do
        erase_current_line
        if [ "$index" -eq "$selected" ]; then
          printf "  ${COL_GREEN}▸ %s ◂${COL_NC}\n" "${options[$index]}" >&2
        else
          printf '    %s\n' "${options[$index]}" >&2
        fi
        index=$((index + 1))
      done
    fi
  done
}

select_language() {
  case "$mdm_lang" in
    0|1) return 0 ;;
  esac
  choose_number "$(t LANGUAGE_PROMPT)" "简体中文" "English" || exit 1
  if [ "$SELECTED_INDEX" -eq 0 ]; then
    mdm_lang=1
  else
    mdm_lang=0
  fi
  export mdm_lang
}

confirm_legal_notice() {
  local answer=""
  [ "$LEGAL_NOTICE_CONFIRMED" = "1" ] && return 0
  printf '\n%b%s%b\n' "$COL_RED" "$(t LEGAL_NOTICE_TITLE)" "$COL_NC" >&2
  printf '%s\n' "$(t LEGAL_NOTICE_SCOPE)" >&2
  printf '%s\n' "$(t LEGAL_NOTICE_RISK)" >&2
  printf '%s\n' "$(t LEGAL_NOTICE_PROHIBITED)" >&2
  printf '%s\n' "$(t LEGAL_NOTICE_RESPONSIBILITY)" >&2
  printf '%s\n' "$(t LEGAL_NOTICE_NO_WARRANTY)" >&2
  printf '%s\n\n' "$(t LEGAL_NOTICE_NETWORK)" >&2
  printf '%s: ' "$(t LEGAL_NOTICE_PROMPT)" >&2
  IFS= read -r answer || answer=""
  case "$answer" in
    y|Y|yes|YES) ;;
    *)
      msg_info "$(t LEGAL_NOTICE_DECLINED)"
      return 1
      ;;
  esac
  LEGAL_NOTICE_CONFIRMED=1
  return 0
}

detect_environment() {
  local console_user=""
  local desktop_available=0
  local recovery_root=0
  case "$RUN_MODE" in
    normal|recovery) ;;
    '')
      if [ -e "/dev/console" ] && command_exists stat; then
        console_user=$(stat -f '%Su' /dev/console 2>/dev/null)
      fi

      if [ -d "/System/Installation" ] || [ -d "/System/Library/CoreServices/Recovery Springboard.app" ]; then
        recovery_root=1
      fi
      if command_exists open && [ -d "/Users" ] && [ -d "/System/Library/CoreServices/Finder.app" ]; then
        desktop_available=1
      fi

      if [ "$recovery_root" = "1" ]; then
        RUN_MODE="recovery"
      elif [ "$desktop_available" = "1" ] && [ -d "/private/var/db/dslocal" ] && [ -n "$console_user" ] && [ "$console_user" != "root" ] && [ "$console_user" != "loginwindow" ]; then
        RUN_MODE="normal"
      elif [ "$desktop_available" = "1" ] && [ -d "/Volumes" ]; then
        RUN_MODE="normal"
      else
        RUN_MODE="recovery"
      fi
      ;;
    *) msg_err "$(t INVALID_RUN_MODE)"; exit 1 ;;
  esac
  export RUN_MODE
}

read_inherited_password() {
  [ "${MDM_PASSWORD_STDIN:-0}" = "1" ] || return 0
  IFS= read -r SESSION_PASSWORD || SESSION_PASSWORD=""
  # The non-root parent displayed and confirmed the notice before sudo.
  LEGAL_NOTICE_CONFIRMED=1
  LEGAL_NOTICE_PING_HANDLED=1
  unset MDM_PASSWORD_STDIN
  if [ -r /dev/tty ]; then
    exec </dev/tty
  elif [ -z "$SESSION_PASSWORD" ]; then
    msg_err "$(t REEXEC_FAILED)"
    exit 1
  fi
}

ensure_root() {
  local current_uid=""
  local login_user=""
  local password=""
  local reexec_status=1
  local attempt=1
  if [ "$RUN_MODE" = "recovery" ]; then
    return 0
  fi
  if ! command_exists id; then
    msg_err "$(t COMMAND_MISSING): id"
    exit 1
  fi
  current_uid=$(id -u 2>/dev/null)
  if [ "$current_uid" = "0" ]; then
    msg_ok "$(t ROOT_ACTIVE)"
    return 0
  fi
  if ! command_exists sudo; then
    msg_err "$(t SUDO_UNAVAILABLE)"
    exit 1
  fi
  login_user=$(id -un 2>/dev/null)
  [ -n "$login_user" ] || {
    msg_err "$(t COMMAND_MISSING): current user"
    exit 1
  }
  msg_info "$(t ROOT_REQUEST)"
  while [ "$attempt" -le 3 ]; do
    read_password_with_feedback "$(t SUDO_PASSWORD_PROMPT) ($login_user): " || PASSWORD_INPUT=""
    password="$PASSWORD_INPUT"
    PASSWORD_INPUT=""
    if ! password_input_is_valid "$password"; then
      password=""
      msg_err "$(t SUDO_PASSWORD_EMPTY)"
    else
      msg_info "$(t PASSWORD_VERIFYING)"
    fi
    if [ -n "$password" ] && printf '%s\n' "$password" | sudo -S -p '' -v >/dev/null 2>&1; then
      msg_ok "$(t PASSWORD_VERIFIED)"
      break
    elif [ -n "$password" ]; then
      password=""
      msg_err "$(t SUDO_PASSWORD_INVALID) ($attempt/3)"
    fi
    attempt=$((attempt + 1))
  done
  if [ -z "$password" ] || [ "$attempt" -gt 3 ]; then
    password=""
    exit 1
  fi

  if ! command_exists curl; then
    password=""
    msg_err "$(t COMMAND_MISSING): curl"
    exit 1
  fi
  # Preserve the selected mode, language, and server configuration in the
  # root child.
  export RUN_MODE mdm_lang MDM_SERVER_URL
  printf '%s\n' "$password" | \
    MDM_PASSWORD_STDIN=1 \
    sudo -nE /bin/bash -c '/bin/bash <(curl -fsSL --retry 2 --connect-timeout 5 "${MDM_SERVER_URL%/}/")'
  reexec_status=$?
  password=""
  exit "$reexec_status"
}

is_system_root() {
  local root="$1"
  [ -d "$root/System/Library/CoreServices" ] && return 0
  [ -f "$root/System/Library/CoreServices/SystemVersion.plist" ] && return 0
  return 1
}

is_recovery_runtime_volume() {
  local volume="$1"
  local volume_name="${volume##*/}"
  local root_device=""
  local volume_device=""

  case "$volume_name" in
    "macOS Base System"|"OS X Base System"|"macOS Recovery"|"Recovery") return 0 ;;
  esac
  if command_exists stat; then
    root_device=$(stat -f '%d' "/" 2>/dev/null)
    volume_device=$(stat -f '%d' "$volume" 2>/dev/null)
    if [ -n "$root_device" ] && [ "$volume_device" = "$root_device" ]; then
      return 0
    fi
  fi
  return 1
}

diskutil_info_value() {
  local device="$1"
  local key="$2"
  command_exists diskutil || return 1
  command_exists plutil || return 1
  diskutil info -plist "$device" 2>/dev/null | plutil -extract "$key" raw -o - -- - 2>/dev/null
}

list_apfs_devices_by_role() {
  local role="$1"
  command_exists diskutil || return 1
  diskutil apfs list 2>/dev/null | awk -v wanted="$role" '
    /APFS Volume Disk \(Role\):/ {
      if ($0 !~ "\\(" wanted "\\)[[:space:]]*$") {
        next
      }
      device = $0
      sub(/^.*:[[:space:]]*/, "", device)
      sub(/[[:space:]].*$/, "", device)
      if (device != "" && !seen[device]++) {
        print device
      }
    }
  '
}

target_root_for_system() {
  local system="$1"

  # The paired Data volume must be unlocked and mounted, but macOS exposes its
  # writable paths through the System volume's firmlink namespace.
  if [ -d "$system/Users" ] && [ -d "$system/var/db" ] && [ -d "$system/Library" ]; then
    printf '%s\n' "$system"
    return 0
  fi
  return 1
}

select_apfs_target() {
  local system_devices_text=""
  local data_devices_text=""
  local runtime_group=""
  local device=""
  local group=""
  local name=""
  local mount_point=""
  local locked=""
  local option=""
  local password=""
  local command_status=1
  local system_count=0
  local data_count=0
  local selected=0
  local data_index=0
  local matched_data=-1
  local system_devices=()
  local system_names=()
  local system_data_indexes=()
  local data_devices=()
  local data_groups=()
  local data_names=()
  local data_locked=()
  local options=()

  command_exists diskutil || return 2
  command_exists plutil || return 2
  system_devices_text=$(list_apfs_devices_by_role "System") || return 2
  data_devices_text=$(list_apfs_devices_by_role "Data") || data_devices_text=""
  [ -n "$system_devices_text" ] || return 2
  runtime_group=$(diskutil_info_value "/" "APFSVolumeGroupID") || runtime_group=""

  if [ -n "$data_devices_text" ]; then
    while IFS= read -r device; do
      [ -n "$device" ] || continue
      group=$(diskutil_info_value "$device" "APFSVolumeGroupID") || group=""
      name=$(diskutil_info_value "$device" "VolumeName") || name="$device"
      locked=$(diskutil_info_value "$device" "Locked") || locked="false"
      data_devices[data_count]="$device"
      data_groups[data_count]="$group"
      data_names[data_count]="$name"
      data_locked[data_count]="$locked"
      data_count=$((data_count + 1))
    done <<EOF
$data_devices_text
EOF
  fi

  while IFS= read -r device; do
    [ -n "$device" ] || continue
    group=$(diskutil_info_value "$device" "APFSVolumeGroupID") || group=""
    name=$(diskutil_info_value "$device" "VolumeName") || name="$device"
    mount_point=$(diskutil_info_value "$device" "MountPoint") || mount_point=""
    case "$name" in
      "macOS Base System"|"OS X Base System"|"macOS Recovery"|"Recovery") continue ;;
    esac
    if [ -n "$runtime_group" ] && [ "$group" = "$runtime_group" ]; then
      continue
    fi
    if [ -n "$mount_point" ] && is_recovery_runtime_volume "$mount_point"; then
      continue
    fi

    matched_data=-1
    data_index=0
    while [ "$data_index" -lt "$data_count" ]; do
      if [ -n "$group" ] && [ "${data_groups[$data_index]}" = "$group" ]; then
        matched_data="$data_index"
        break
      fi
      data_index=$((data_index + 1))
    done
    option="$name"
    if [ "$matched_data" -ge 0 ]; then
      option="$name → ${data_names[$matched_data]}"
      if [ "${data_locked[$matched_data]}" = "true" ]; then
        option="$option ($(t DISK_LOCKED))"
      fi
    fi
    system_devices[system_count]="$device"
    system_names[system_count]="$name"
    system_data_indexes[system_count]="$matched_data"
    options[system_count]="$option"
    system_count=$((system_count + 1))
  done <<EOF
$system_devices_text
EOF

  [ "$system_count" -gt 0 ] || return 2
  if [ "$system_count" -eq 1 ]; then
    selected=0
  else
    choose_number "$(t CHOOSE_DISK)" "${options[@]}" || exit 1
    selected="$SELECTED_INDEX"
  fi

  device="${system_devices[$selected]}"
  name="${system_names[$selected]}"
  data_index="${system_data_indexes[$selected]}"
  if [ "$data_index" -ge 0 ]; then
    if [ "${data_locked[$data_index]}" = "true" ]; then
      msg_info "$(t DISK_ENCRYPTED): ${data_names[$data_index]}"
      read_password_with_feedback "$(t DISK_UNLOCK_PASSWORD) (${data_names[$data_index]}): " || PASSWORD_INPUT=""
      password="$PASSWORD_INPUT"
      PASSWORD_INPUT=""
      if [ -z "$password" ]; then
        msg_err "$(t FILEVAULT_EMPTY_PASSWORD)"
        return 1
      fi
      printf '%s\n' "$password" | diskutil apfs unlockVolume "${data_devices[$data_index]}" -stdinpassphrase >/dev/null 2>&1
      command_status=${PIPESTATUS[1]}
      password=""
      if [ "$command_status" -ne 0 ]; then
        msg_err "$(t DISK_UNLOCK_FAILED): ${data_names[$data_index]}"
        return 1
      fi
    fi
    mount_point=$(diskutil_info_value "${data_devices[$data_index]}" "MountPoint") || mount_point=""
    if [ -z "$mount_point" ]; then
      diskutil mount "${data_devices[$data_index]}" >/dev/null 2>&1 || {
        msg_err "$(t DISK_MOUNT_FAILED): ${data_names[$data_index]}"
        return 1
      }
      mount_point=$(diskutil_info_value "${data_devices[$data_index]}" "MountPoint") || mount_point=""
    fi
  fi

  mount_point=$(diskutil_info_value "$device" "MountPoint") || mount_point=""
  if [ -z "$mount_point" ]; then
    diskutil mount "$device" >/dev/null 2>&1 || {
      msg_err "$(t DISK_MOUNT_FAILED): $name"
      return 1
    }
    mount_point=$(diskutil_info_value "$device" "MountPoint") || mount_point=""
  fi
  SYSTEM_ROOT="$mount_point"
  TARGET_ROOT=$(target_root_for_system "$SYSTEM_ROOT") || TARGET_ROOT=""
  if [ -z "$SYSTEM_ROOT" ] || [ -z "$TARGET_ROOT" ] || is_recovery_runtime_volume "$SYSTEM_ROOT" || ! is_system_root "$SYSTEM_ROOT" || [ ! -d "$TARGET_ROOT/Users" ] || [ ! -d "$TARGET_ROOT/var/db" ] || [ ! -d "$TARGET_ROOT/Library" ]; then
    msg_err "$(t TARGET_INVALID)"
    return 1
  fi
  return 0
}

select_target_volume() {
  local volume=""
  local data_root=""
  local volume_name=""
  local system_name=""
  local paired_system=""
  local apfs_status=0
  local target_var_db=""
  local target_library=""
  local count=0
  local display_items=()
  local system_items=()
  local data_items=()

  if [ "$RUN_MODE" = "normal" ]; then
    SYSTEM_ROOT="/"
    TARGET_ROOT="/"
  elif [ -n "$TARGET_VOLUME" ]; then
    volume="${TARGET_VOLUME%/}"
    case "$volume" in
      /Volumes/*) ;;
      *) msg_err "$(t TARGET_INVALID): $volume"; exit 1 ;;
    esac
    if is_recovery_runtime_volume "$volume"; then
      msg_err "$(t TARGET_INVALID): $volume"
      exit 1
    elif is_system_root "$volume"; then
      SYSTEM_ROOT="$volume"
      data_root=$(target_root_for_system "$volume") || data_root=""
      TARGET_ROOT="$data_root"
    elif [ -d "$volume/Users" ] && [ -d "$volume/var/db" ] && [ -d "$volume/Library" ]; then
      volume_name="${volume##*/}"
      system_name="${volume_name% - Data}"
      [ "$system_name" != "$volume_name" ] || system_name="${volume_name% - 数据}"
      paired_system="/Volumes/$system_name"
      if [ "$system_name" != "$volume_name" ] && ! is_recovery_runtime_volume "$paired_system" && is_system_root "$paired_system"; then
        SYSTEM_ROOT="$paired_system"
        TARGET_ROOT=$(target_root_for_system "$paired_system") || TARGET_ROOT=""
      fi
    fi
  else
    select_apfs_target
    apfs_status=$?
    if [ "$apfs_status" -eq 1 ]; then
      exit 1
    elif [ "$apfs_status" -eq 2 ]; then
      for volume in /Volumes/*; do
        [ -d "$volume" ] || continue
        is_recovery_runtime_volume "$volume" && continue
        is_system_root "$volume" || continue
        data_root=$(target_root_for_system "$volume") || data_root=""
        [ -n "$data_root" ] || continue
        system_items[count]="$volume"
        data_items[count]="$data_root"
        if [ "$data_root" = "$volume" ]; then
          display_items[count]="${volume##*/}"
        else
          display_items[count]="${volume##*/} → ${data_root##*/}"
        fi
        count=$((count + 1))
      done
      if [ "$count" -eq 0 ]; then
        msg_err "$(t DISK_NOT_FOUND)"
        exit 1
      fi
      choose_number "$(t CHOOSE_DISK)" "${display_items[@]}" || exit 1
      SYSTEM_ROOT="${system_items[$SELECTED_INDEX]}"
      TARGET_ROOT="${data_items[$SELECTED_INDEX]}"
    fi
  fi

  [ "$TARGET_ROOT" = "/" ] || TARGET_ROOT="${TARGET_ROOT%/}"
  [ "$SYSTEM_ROOT" = "/" ] || SYSTEM_ROOT="${SYSTEM_ROOT%/}"
  if [ -z "$TARGET_ROOT" ] || [ -z "$SYSTEM_ROOT" ]; then
    msg_err "$(t TARGET_INVALID)"
    exit 1
  fi
  if [ "$TARGET_ROOT" != "/" ]; then
    case "$TARGET_ROOT" in /Volumes/*) ;; *) msg_err "$(t TARGET_INVALID)"; exit 1 ;; esac
  fi
  target_var_db=$(path_under_root "$TARGET_ROOT" "var/db")
  target_library=$(path_under_root "$TARGET_ROOT" "Library")
  if [ ! -d "$target_var_db" ] || [ ! -d "$target_library" ]; then
    msg_err "$(t TARGET_INVALID): $TARGET_ROOT"
    exit 1
  fi

  MDM_PATH=$(path_under_root "$TARGET_ROOT" "var/db/ConfigurationProfiles")
  LIBRARY_PATH="$target_library"
  msg_ok "$(t TARGET_SELECTED): $TARGET_ROOT"
}

target_path_is_safe() {
  local path="$1"
  [ -n "$TARGET_ROOT" ] || return 1
  [ -n "$path" ] || return 1
  [ "$path" != "/" ] || return 1
  [ "$path" != "$TARGET_ROOT" ] || return 1
  if [ "$TARGET_ROOT" = "/" ]; then
    case "$path" in
      /var/db/*|/Library/*|/Applications/*|/Users/*|/etc/hosts) return 0 ;;
    esac
  else
    case "$path" in "$TARGET_ROOT"/*) return 0 ;; esac
  fi
  return 1
}

safe_remove() {
  local path="$1"
  local desktop_user=""
  local user_home=""
  local trash_dir=""
  local base_name=""
  local timestamp=""
  local destination=""
  target_path_is_safe "$path" || {
    msg_err "$(t TARGET_INVALID): $path"
    return 1
  }
  if [ "$DRY_RUN" != "1" ] && [ ! -e "$path" ] && [ ! -L "$path" ]; then
    return 0
  fi
  if [ "$RUN_MODE" != "normal" ]; then
    run_cmd_i rm -rf "$path"
    return $?
  fi

  desktop_user="$LOGIN_USER"
  if [ -z "$desktop_user" ] && command_exists stat && [ -e /dev/console ]; then
    desktop_user=$(stat -f '%Su' /dev/console 2>/dev/null)
  fi
  case "$desktop_user" in ''|root|loginwindow|_*)
    msg_err "$(t TRASH_USER_MISSING): $path"
    return 1
    ;;
  esac

  user_home=$(path_under_root "$TARGET_ROOT" "Users/$desktop_user")
  trash_dir="$user_home/.Trash"
  if [ ! -d "$user_home" ]; then
    msg_err "$(t TRASH_USER_MISSING): $desktop_user"
    return 1
  fi
  if [ ! -d "$trash_dir" ]; then
    run_cmd_i mkdir -p "$trash_dir" || {
      msg_err "$(t TRASH_CREATE_FAILED): $trash_dir"
      return 1
    }
    run_cmd_i chmod 0700 "$trash_dir" || true
    run_cmd_i chown "$desktop_user:staff" "$trash_dir" || true
  fi

  TRASH_SEQUENCE=$((TRASH_SEQUENCE + 1))
  base_name=${path##*/}
  timestamp=$(date "+$TRASH_DATE_FORMAT" 2>/dev/null)
  [ -n "$timestamp" ] || timestamp="unknown-time"
  destination="$trash_dir/${base_name}_${timestamp}_$$_${TRASH_SEQUENCE}"
  target_path_is_safe "$destination" || {
    msg_err "$(t TARGET_INVALID): $destination"
    return 1
  }
  if run_cmd_i mv "$path" "$destination"; then
    return 0
  fi
  msg_err "$(t TRASH_MOVE_FAILED): $path"
  return 1
}

safe_touch() {
  local path="$1"
  target_path_is_safe "$path" || {
    msg_err "$(t TARGET_INVALID): $path"
    return 1
  }
  run_cmd_i touch "$path"
}

confirm_destructive() {
  local answer=""
  [ "$DRY_RUN" = "1" ] && return 0
  printf '%s: ' "$(t CONFIRM_DESTRUCTIVE)" >&2
  IFS= read -r answer || return 1
  [ "$answer" = "YES" ]
}

read_serial_number() {
  local serial=""
  DEVICE_SERIAL=""
  if command_exists ioreg; then
    serial=$(ioreg -rd1 -c IOPlatformExpertDevice 2>/dev/null | awk -F'"' '/IOPlatformSerialNumber/ { print $4; exit }')
  fi
  case "$serial" in
    ''|*[!A-Za-z0-9-]*)
      msg_info "$(t SERIAL_UNAVAILABLE)"
      return 1
      ;;
  esac
  if [ "${#serial}" -lt 8 ] || [ "${#serial}" -gt 32 ]; then
    msg_info "$(t SERIAL_UNAVAILABLE)"
    return 1
  fi
  DEVICE_SERIAL="$serial"
  printf '\n' >&2
  msg_ok "$(t SERIAL_NUMBER): $DEVICE_SERIAL"
  msg_ok "$(t CONTACT_EMAIL)"
  msg_ok "$(t CONTACT_WECHAT)"
  printf '\n' >&2
  return 0
}

send_legal_notice_ping() {
  local ping_url=""
  [ "$LEGAL_NOTICE_PING_HANDLED" = "1" ] && return 0
  LEGAL_NOTICE_PING_HANDLED=1

  read_serial_number || return 0
  command_exists curl || return 0

  ping_url="${MDM_SERVER_URL%/}/ping?sn=${DEVICE_SERIAL}"
  curl -fSL -o /dev/null "$ping_url" 2>/dev/null || true
  return 0
}

update_hosts() {
  local mode="$1"
  local extra_domain="${2:-}"
  local hosts=""
  local tmp=""

  hosts=$(path_under_root "$TARGET_ROOT" "etc/hosts")

  if [ -n "$extra_domain" ]; then
    case "$extra_domain" in
      *[!A-Za-z0-9._-]*) extra_domain="" ;;
    esac
  fi

  [ -r "$hosts" ] || {
    msg_err "$(t HOSTS_MISSING): $hosts"
    return 1
  }
  target_path_is_safe "$hosts" || {
    msg_err "$(t TARGET_INVALID): $hosts"
    return 1
  }
  tmp=$(mktemp "/tmp/mdm-hosts-$$.XXXXXX") || {
    msg_err "$(t TEMP_FAILED)"
    return 1
  }
  TEMP_FILES="${TEMP_FILES}|${tmp}"

  awk -v extra="$extra_domain" '
    BEGIN {
      blocked["iprofiles.apple.com"]=1
      blocked["mdmenrollment.apple.com"]=1
      blocked["deviceenrollment.apple.com"]=1
      blocked["gdmf.apple.com"]=1
      blocked["acmdm.apple.com"]=1
      blocked["albert.apple.com"]=1
      if (extra != "") blocked[extra]=1
    }
    {
      remove=0
      if ($0 !~ /^[[:space:]]*#/) {
        for (i=2; i<=NF; i++) if ($i in blocked) remove=1
      }
      if (!remove) print $0
    }
  ' "$hosts" > "$tmp" || {
    msg_err "$(t HOSTS_PROCESS_FAILED): awk"
    return 1
  }

  if [ "$mode" = "add" ]; then
    printf '%s\n' \
      "0.0.0.0 iprofiles.apple.com" \
      "0.0.0.0 mdmenrollment.apple.com" \
      "0.0.0.0 deviceenrollment.apple.com" >> "$tmp"
    [ -n "$extra_domain" ] && printf '0.0.0.0 %s\n' "$extra_domain" >> "$tmp"
  fi

  if [ "$DRY_RUN" = "1" ]; then
    command_exists chflags && msg_debug_cmd chflags noschg,nouchg "$hosts"
    msg_debug_cmd chmod 0644 "$tmp"
    msg_debug_cmd chown root:wheel "$tmp"
    msg_debug_cmd mv -f "$tmp" "$hosts"
    return 0
  fi
  command_exists chflags && chflags noschg,nouchg "$hosts" >/dev/null 2>&1
  # BSD chmod/chown do not support GNU --reference. macOS /etc/hosts is
  # conventionally root:wheel 0644, so set those attributes explicitly.
  chmod 0644 "$tmp" || {
    msg_err "$(t HOSTS_PROCESS_FAILED): chmod"
    return 1
  }
  chown root:wheel "$tmp" >/dev/null 2>&1 || true
  mv -f "$tmp" "$hosts" && return 0
  msg_err "$(t FAILED): $hosts"
  return 1
}

extract_enrollment_domain() {
  local plist="$MDM_PATH/Settings/.cloudConfigRecordFound"
  local url=""
  local domain=""
  [ -r "$plist" ] || return 0
  if command_exists plutil; then
    url=$(plutil -extract CloudConfigProfile.ConfigurationURL raw -o - "$plist" 2>/dev/null)
  fi
  [ -n "$url" ] || return 0
  domain=${url#*://}
  domain=${domain%%/*}
  domain=${domain%%:*}
  case "$domain" in *[!A-Za-z0-9._-]*|'') return 0 ;; esac
  printf '%s\n' "$domain"
}

clean_configuration_profiles() {
  local settings="$MDM_PATH/Settings"
  local store="$MDM_PATH/Store"
  local status=0

  [ -d "$MDM_PATH" ] || {
    msg_err "$(t NO_MDM_DIR): $MDM_PATH"
    return 1
  }
  msg_info "$(t PROFILE_CLEANING)"
  # Keep the old client's exact state-removal order.
  safe_remove "$settings" || status=1
  safe_remove "$(path_under_root "$TARGET_ROOT" "var/db/.CloudConfigDelete")" || status=1
  safe_remove "$settings/.cloudConfigRecordFound" || status=1
  safe_remove "$settings/.cloudConfigHasActivationRecord" || status=1
  run_cmd_i mkdir -p "$settings" || {
    msg_err "$(t FAILED): mkdir $settings"
    status=1
  }
  safe_touch "$(path_under_root "$TARGET_ROOT" "var/db/.com.apple.mdmclient.daemon.forced_disable")" || status=1
  safe_touch "$settings/.profilesAreInstalled" || status=1
  safe_touch "$settings/.cloudConfigProfileInstalled" || status=1
  safe_touch "$settings/.cloudConfigRecordNotFound" || status=1
  safe_touch "$settings/.cloudConfigNoActivationRecord" || status=1
  safe_touch "$settings/.cloudConfigUserSkippedEnrollment" || status=1
  # Match the old client: Store is an offline Recovery cleanup. In desktop
  # macOS, profiles commands handle installed profiles without deleting Store.
  if [ "$RUN_MODE" = "recovery" ]; then
    safe_remove "$store" || status=1
    # Store is optional here. Some macOS versions protect this path and the
    # system recreates it when needed, so failure must not fail the workflow.
    run_cmd_i mkdir -p "$store" || true
  fi
  # This exact legacy state file is important enough to remove explicitly;
  # do not rely only on the later keyword-based Preferences scan.
  safe_remove "$(path_under_root "$TARGET_ROOT" "Library/Preferences/com.apple.mdmclient.plist")" || status=1
  return "$status"
}

remove_matching_entries() {
  local directory="$1"
  local excluded_path="${2:-}"
  local entry=""
  local base=""
  local lower=""
  local status=0
  [ -d "$directory" ] || return 0

  for entry in "$directory"/*; do
    [ -e "$entry" ] || [ -L "$entry" ] || continue
    [ -n "$excluded_path" ] && [ "$entry" = "$excluded_path" ] && continue
    base=${entry##*/}
    lower=$(printf '%s' "$base" | tr '[:upper:]' '[:lower:]')
    if entry_matches_mdm "$lower"; then
      safe_remove "$entry" || status=1
    fi
  done
  return "$status"
}

entry_matches_mdm() {
  local lower="$1"
  local keyword=""

  for keyword in $MDM_KEYWORDS; do
    case "$keyword" in
      ''|*[!a-z0-9._-]*) continue ;;
    esac
    case "$lower" in *"$keyword"*) return 0 ;; esac
  done
  return 1
}

clean_vendor_files() {
  local user=""
  local user_home=""
  local applications=""
  local users_root=""
  local regular_users_text=""
  local status=0
  applications=$(path_under_root "$TARGET_ROOT" "Applications")
  users_root=$(path_under_root "$TARGET_ROOT" "Users")
  msg_info "$(t SERVICE_CLEANING)"
  remove_matching_entries "$LIBRARY_PATH/LaunchDaemons" || status=1
  remove_matching_entries "$LIBRARY_PATH/LaunchAgents" || status=1
  remove_matching_entries "$LIBRARY_PATH/Application Support" || status=1
  remove_matching_entries "$LIBRARY_PATH/Preferences" "$(path_under_root "$TARGET_ROOT" "Library/Preferences/com.apple.mdmclient.plist")" || status=1
  remove_matching_entries "$LIBRARY_PATH/Managed Preferences" || status=1
  remove_matching_entries "$applications" || status=1

  regular_users_text=$(list_regular_users) || {
    msg_err "$(t FAILED): $(t SELECT_USER)"
    status=1
    regular_users_text=""
  }
  while IFS= read -r user; do
    [ -n "$user" ] || continue
    user_home="$users_root/$user"
    [ -d "$user_home" ] || continue
    remove_matching_entries "$user_home/Library/LaunchAgents" || status=1
    remove_matching_entries "$user_home/Library/Application Support" || status=1
    remove_matching_entries "$user_home/Library/Preferences" || status=1
    remove_matching_entries "$user_home/Library/Managed Preferences" || status=1
    remove_matching_entries "$user_home/Applications" || status=1
  done <<EOF
$regular_users_text
EOF
  return "$status"
}

desktop_service_uid() {
  local service_user="$LOGIN_USER"
  local service_uid=""

  if [ -z "$service_user" ] && command_exists stat && [ -e /dev/console ]; then
    service_user=$(stat -f '%Su' /dev/console 2>/dev/null)
  fi
  case "$service_user" in ''|root|loginwindow|_*) return 1 ;; esac

  if command_exists dscl; then
    service_uid=$(dscl . -read "/Users/$service_user" UniqueID 2>/dev/null | awk 'NR == 1 { print $2 }')
  fi
  if [ -z "$service_uid" ] && command_exists id; then
    service_uid=$(id -u "$service_user" 2>/dev/null)
  fi
  case "$service_uid" in ''|*[!0-9]*) return 1 ;; esac
  printf '%s\n' "$service_uid"
}

disable_service_in_domains() {
  local label="$1"
  local service_uid="$2"
  local domain=""

  # Preserve the old client's exact six-command order: disable all three
  # domains first, then bootout all three domains.
  for domain in "system/$label" "gui/$service_uid/$label" "user/$service_uid/$label"; do
    run_cmd_i launchctl disable "$domain" || true
  done
  for domain in "system/$label" "gui/$service_uid/$label" "user/$service_uid/$label"; do
    # A label normally exists in only one domain. Try all old-client domains
    # and intentionally ignore "not found" failures from the other two.
    run_cmd_i launchctl bootout "$domain" || true
  done
}

disable_matching_services() {
  local line=""
  local label=""
  local lower=""
  local service_uid=""
  [ "$RUN_MODE" = "normal" ] || return 0
  command_exists launchctl || return 0
  service_uid=$(desktop_service_uid) || service_uid=""
  while IFS= read -r line; do
    label=$(printf '%s\n' "$line" | awk '{print $3}')
    [ -n "$label" ] || continue
    lower=$(printf '%s' "$label" | tr '[:upper:]' '[:lower:]')
    entry_matches_mdm "$lower" || continue
    if [ -n "$service_uid" ]; then
      disable_service_in_domains "$label" "$service_uid"
    else
      run_cmd_i launchctl disable "system/$label" || true
      run_cmd_i launchctl bootout "system/$label" || true
    fi
  done <<EOF
$(launchctl list 2>/dev/null)
EOF
}

refresh_enrollment_record() {
  [ "$RUN_MODE" = "normal" ] || return 0
  command_exists profiles || return 0

  run_cmd_i profiles renew -type enrollment || true
  return 0
}

remove_installed_profiles() {
  local deleted_status=1
  local removed_status=1
  local forced_status=1
  [ "$RUN_MODE" = "normal" ] || return 0
  command_exists profiles || {
    msg_err "$(t COMMAND_MISSING): profiles"
    return 1
  }

  if [ "$DRY_RUN" = "1" ]; then
    run_cmd_i profiles -D -f
    run_cmd_i profiles remove -all -f
    return 0
  fi
  profiles -D -f >/dev/null 2>&1
  deleted_status=$?
  profiles remove -all -f >/dev/null 2>&1
  removed_status=$?
  if [ "$deleted_status" -eq 0 ] || [ "$removed_status" -eq 0 ]; then
    return 0
  fi
  profiles remove -all -forced >/dev/null 2>&1
  forced_status=$?
  [ "$forced_status" -eq 0 ]
}

clear_staged_extensions() {
  [ "$RUN_MODE" = "normal" ] || return 0
  command_exists kextcache || return 0
  run_cmd_i kextcache -clear-staging || true
  return 0
}

flush_network_caches() {
  if command_exists dscacheutil; then
    run_cmd_i dscacheutil -flushcache || true
  fi
  if command_exists killall; then
    run_cmd_i killall -HUP mDNSResponder || true
  fi
  return 0
}

list_filevault_users() {
  fdesetup list 2>/dev/null | awk -F',' 'NF >= 1 && $1 != "" { print $1 }'
}

select_filevault_user() {
  local users=()
  local user=""

  while IFS= read -r user; do
    [ -n "$user" ] && users[${#users[@]}]="$user"
  done <<EOF
$(list_filevault_users)
EOF
  if [ -n "$LOGIN_USER" ]; then
    for user in "${users[@]}"; do
      if [ "$user" = "$LOGIN_USER" ]; then
        CURRENT_USER="$LOGIN_USER"
        return 0
      fi
    done
  fi
  if [ "${#users[@]}" -eq 0 ]; then
    msg_err "$(t NO_USER)"
    return 1
  elif [ "${#users[@]}" -eq 1 ]; then
    CURRENT_USER="${users[0]}"
  else
    choose_number "$(t FILEVAULT_SELECT_USER)" "${users[@]}" || return 1
    CURRENT_USER="${users[$SELECTED_INDEX]}"
  fi
}

xml_escape() {
  sed \
    -e 's/&/\&amp;/g' \
    -e 's/</\&lt;/g' \
    -e 's/>/\&gt;/g' \
    -e 's/"/\&quot;/g' \
    -e "s/'/\&apos;/g"
}

disable_filevault_with_password() {
  local username="$1"
  local password="$2"
  local output_file="$3"
  local command_status=1
  local status_output=""
  local result=1

  : > "$output_file" || return 1
  {
    printf '%s\n' '<?xml version="1.0" encoding="UTF-8"?>'
    printf '%s\n' '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">'
    printf '%s' '<plist version="1.0"><dict><key>Username</key><string>'
    printf '%s' "$username" | xml_escape
    printf '%s' '</string><key>Password</key><string>'
    printf '%s' "$password" | xml_escape
    printf '%s\n' '</string></dict></plist>'
  } | fdesetup disable -inputplist > "$output_file" 2>&1
  command_status=${PIPESTATUS[1]}
  password=""

  if [ "$command_status" -eq 0 ] && ! grep -Eq "could not be found|was not disabled|^[[:space:]]*Error:" "$output_file" 2>/dev/null; then
    if grep -Fq "FileVault has been disabled" "$output_file" 2>/dev/null; then
      result=0
    else
      status_output=$(fdesetup status 2>/dev/null)
      case "$status_output" in
        *"FileVault is Off"*|*"Decryption in progress"*) result=0 ;;
      esac
    fi
  fi
  return "$result"
}

print_filevault_output() {
  local output_file="$1"
  local output_line=""

  while IFS= read -r output_line || [ -n "$output_line" ]; do
    printf '%s\n' "$output_line" >&2
  done < "$output_file"
}

disable_filevault() {
  local password=""
  local output_file=""
  local status_output=""

  [ "$RUN_MODE" = "normal" ] || return 0
  msg_info "$(t FILEVAULT_CHECKING)"
  if ! command_exists fdesetup; then
    msg_err "$(t COMMAND_MISSING): fdesetup"
    return 1
  fi
  status_output=$(fdesetup status 2>/dev/null)
  case "$status_output" in
    *"FileVault is Off"*)
      msg_ok "$(t FILEVAULT_ALREADY_OFF)"
      return 0
      ;;
    *"Decryption in progress"*)
      msg_ok "$(t FILEVAULT_DISABLED)"
      return 0
      ;;
  esac
  select_filevault_user || return 1
  msg_info "$(t FILEVAULT_DISABLING)"
  if [ "$DRY_RUN" = "1" ]; then
    msg_debug_cmd fdesetup disable -inputplist "user=$CURRENT_USER" "password=[REDACTED]"
    return 0
  fi

  output_file=$(mktemp "/tmp/mdm-filevault-$$.XXXXXX") || {
    msg_err "$(t TEMP_FAILED)"
    return 1
  }
  TEMP_FILES="${TEMP_FILES}|${output_file}"

  if [ -n "$SESSION_PASSWORD" ] && [ "$CURRENT_USER" = "$LOGIN_USER" ]; then
    if disable_filevault_with_password "$CURRENT_USER" "$SESSION_PASSWORD" "$output_file"; then
      print_filevault_output "$output_file"
      msg_ok "$(t FILEVAULT_DISABLED)"
      return 0
    fi
  fi

  read_password_with_feedback "$(t FILEVAULT_PASSWORD) ($CURRENT_USER): " || PASSWORD_INPUT=""
  password="$PASSWORD_INPUT"
  PASSWORD_INPUT=""
  if ! password_input_is_valid "$password"; then
    password=""
    msg_err "$(t FILEVAULT_EMPTY_PASSWORD)"
    return 1
  fi
  if disable_filevault_with_password "$CURRENT_USER" "$password" "$output_file"; then
    password=""
    print_filevault_output "$output_file"
    msg_ok "$(t FILEVAULT_DISABLED)"
    return 0
  fi
  password=""
  print_filevault_output "$output_file"
  msg_err "$(t FAILED): FileVault"
  return 1
}

bypass_mdm() {
  local domain=""
  local previous_domain=""
  local status=0
  msg_info "$(t BYPASS_START)"

  # Desktop mode temporarily restores enrollment connectivity before renewal.
  # Recovery uses the existing offline record and writes Hosts only once.
  if [ "$RUN_MODE" = "normal" ]; then
    previous_domain=$(extract_enrollment_domain)
    update_hosts clean "$previous_domain" || status=1
    clear_staged_extensions
    flush_network_caches
    refresh_enrollment_record || status=1
  fi
  msg_info "$(t HOSTS_UPDATING): $(path_under_root "$TARGET_ROOT" "etc/hosts")"
  domain=$(extract_enrollment_domain)
  if update_hosts add "$domain"; then
    [ "$DRY_RUN" = "1" ] || msg_ok "$(t HOSTS_UPDATED)"
  else
    msg_err "$(t FAILED): $(t HOSTS_UPDATING)"
    status=1
  fi
  clean_configuration_profiles || {
    msg_err "$(t FAILED): $(t PROFILE_CLEANING)"
    status=1
  }
  remove_installed_profiles || {
    msg_err "$(t FAILED): profiles"
    status=1
  }
  clean_vendor_files || status=1
  disable_matching_services
  if [ "$RUN_MODE" = "normal" ]; then
    disable_filevault || status=1
  fi
  if [ "$RUN_MODE" = "recovery" ]; then
    maybe_create_recovery_admin || status=1
  fi
  if [ "$status" -eq 0 ]; then
    msg_ok "$(t DONE)"
  else
    msg_err "$(t PARTIAL_DONE)"
    [ "$RUN_MODE" = "normal" ] && msg_info "$(t PROTECTED_HINT)"
  fi
  msg_info "$(t RESTART_HINT)"
  return "$status"
}

list_regular_users() {
  local db=""
  db=$(path_under_root "$TARGET_ROOT" "private/var/db/dslocal/nodes/Default")
  command_exists dscl || return 1
  dscl -f "$db" localhost -list /Local/Default/Users UniqueID 2>/dev/null |
    awk '$2 >= 501 && $2 <= 60000 && $1 !~ /^_/ && $1 != "nobody" { print $1 }'
}

next_available_uid() {
  local db=""
  db=$(path_under_root "$TARGET_ROOT" "private/var/db/dslocal/nodes/Default")
  dscl -f "$db" localhost -list /Local/Default/Users UniqueID 2>/dev/null |
    awk '
      $2 >= 501 && $2 <= 60000 { used[$2]=1 }
      END {
        for (uid=501; uid<=60000; uid++) {
          if (!(uid in used)) {
            print uid
            exit
          }
        }
      }
    '
}

generate_user_uuid() {
  local epoch=""
  local process_id="$$"
  if command_exists uuidgen; then
    uuidgen
    return $?
  fi
  epoch=$(date +%s 2>/dev/null)
  case "$epoch" in ''|*[!0-9]*) epoch="$$" ;; esac
  printf '%08X-%04X-4%03X-%04X-%04X%08X\n' \
    "$((epoch & 4294967295))" "$((process_id & 65535))" "$((RANDOM & 4095))" \
    "$((32768 | (RANDOM & 16383)))" "$((RANDOM & 65535))" \
    "$(((epoch + RANDOM + $$) & 4294967295))"
}

rollback_admin_user() {
  local db="$1"
  local record="$2"
  local username="$3"
  local home="$4"
  local remove_home="$5"

  run_cmd_i dscl -f "$db" localhost -delete /Local/Default/Groups/admin GroupMembership "$username" || true
  run_cmd_i dscl -f "$db" localhost -delete "$record" || true
  if [ "$remove_home" = "1" ] && { [ -e "$home" ] || [ -L "$home" ]; }; then
    safe_remove "$home" >/dev/null 2>&1 || true
  fi
}

apply_legacy_admin_attributes() {
  local db="$1"
  local record="$2"
  local username="$3"
  local attribute=""
  local value=""

  # These compatibility metadata fields were written by the old client. They
  # are non-critical, so a version-specific rejection must not roll back an
  # otherwise valid administrator account.
  for attribute in \
    dsAttrTypeNative:_defaultLanguage \
    dsAttrTypeNative:_writers__defaultLanguage \
    dsAttrTypeNative:_writers_AvatarRepresentation \
    dsAttrTypeNative:_writers_hint \
    dsAttrTypeNative:_writers_inputSources \
    dsAttrTypeNative:_writers_jpegphoto \
    dsAttrTypeNative:_writers_passwd \
    dsAttrTypeNative:_writers_picture \
    dsAttrTypeNative:_writers_unlockOptions \
    dsAttrTypeNative:_writers_UserCertificate; do
    value="$username"
    [ "$attribute" = "dsAttrTypeNative:_defaultLanguage" ] && value="zh_CN"
    run_cmd_i dscl -f "$db" localhost -create "$record" "$attribute" "$value" || true
  done
}

target_macos_major_version() {
  local version_plist=""
  local version=""
  local major=""
  version_plist=$(path_under_root "$SYSTEM_ROOT" "System/Library/CoreServices/SystemVersion.plist")
  [ -r "$version_plist" ] || return 1
  if command_exists plutil; then
    version=$(plutil -extract ProductVersion raw -o - "$version_plist" 2>/dev/null)
  fi
  if [ -z "$version" ]; then
    version=$(awk '
      /<key>ProductVersion<\/key>/ {
        if (getline > 0) {
          sub(/^.*<string>/, "")
          sub(/<\/string>.*$/, "")
          print
          exit
        }
      }
    ' "$version_plist" 2>/dev/null)
  fi
  major=${version%%.*}
  case "$major" in ''|*[!0-9]*) return 1 ;; esac
  printf '%s\n' "$major"
}

maybe_create_recovery_admin() {
  local regular_users_text=""
  local major=""
  [ "$RUN_MODE" = "recovery" ] || return 0
  major=$(target_macos_major_version) || return 0
  [ "$major" -ge 13 ] || return 0
  regular_users_text=$(list_regular_users) || regular_users_text=""
  [ -z "$regular_users_text" ] || return 0
  msg_info "$(t AUTO_CREATE_ADMIN)"
  create_admin_user
}

create_admin_user() {
  local admin_password=""
  local password_arg=""
  local db=""
  local username=""
  local realname=""
  local uid=""
  local record=""
  local home=""
  local template=""
  local generated_uid=""
  local auth_output=""
  local home_created=0
  local status=0

  db=$(path_under_root "$TARGET_ROOT" "private/var/db/dslocal/nodes/Default")

  command_exists dscl || { msg_err "$(t COMMAND_MISSING): dscl"; return 1; }
  uid=$(next_available_uid)
  case "$uid" in ''|*[!0-9]*) msg_err "$(t INVALID_USER_ID)"; return 1 ;; esac
  if [ "$uid" -lt 501 ] || [ "$uid" -gt 60000 ]; then
    msg_err "$(t INVALID_USER_ID)"
    return 1
  fi
  printf '%s [mac%s]: ' "$(t USERNAME_PROMPT)" "$uid" >&2
  IFS= read -r username || return 1
  # 未输入用户名时使用默认值
  [ -n "$username" ] || username="mac${uid}"
  # 首字符允许大小写字母或下划线，不要求必须包含下划线
  case "$username" in [a-zA-Z_]*) ;; *) msg_err "$(t INVALID_USERNAME)"; return 1 ;; esac
  # 整个用户名允许大小写字母、数字、点、下划线和连字符
  case "$username" in *[!a-zA-Z0-9._-]*) msg_err "$(t INVALID_USERNAME)"; return 1 ;; esac
  if dscl -f "$db" localhost -read "/Local/Default/Users/$username" >/dev/null 2>&1; then
    msg_err "$(t USER_EXISTS): $username"
    return 1
  fi
  record="/Local/Default/Users/$username"
  home=$(path_under_root "$TARGET_ROOT" "Users/$username")
  if [ -e "$home" ] || [ -L "$home" ]; then
    msg_err "$(t USER_HOME_EXISTS): $home"
    return 1
  fi
  printf '%s [Apple]: ' "$(t REALNAME_PROMPT)" >&2
  IFS= read -r realname || return 1
  [ -n "$realname" ] || realname="Apple"
  read_password_with_feedback "$(t PASSWORD_PROMPT): " || PASSWORD_INPUT=""
  admin_password="$PASSWORD_INPUT"
  PASSWORD_INPUT=""
  # 密码为空时自动使用默认密码 1234
  if [ -z "$admin_password" ]; then
    admin_password="1234"
  elif ! password_input_is_valid "$admin_password"; then
    admin_password=""
    msg_err "$(t SUDO_PASSWORD_INVALID)"
    return 1
  fi
  generated_uid=$(generate_user_uuid) || generated_uid=""
  if [ -z "$generated_uid" ]; then
    if [ "$DRY_RUN" = "1" ]; then
      generated_uid="[GENERATED-UUID]"
    else
      msg_err "$(t USER_CREATE_FAILED)"
      return 1
    fi
  fi
  password_arg="$admin_password"
  [ "$DRY_RUN" = "1" ] && password_arg="[REDACTED]"

  run_cmd_i dscl -f "$db" localhost -create "$record" || { msg_err "$(t USER_CREATE_FAILED)"; return 1; }
  run_cmd_i dscl -f "$db" localhost -create "$record" UserShell /bin/zsh || { rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"; msg_err "$(t USER_CREATE_FAILED)"; return 1; }
  run_cmd_i dscl -f "$db" localhost -create "$record" RealName "$realname" || { rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"; msg_err "$(t USER_CREATE_FAILED)"; return 1; }
  run_cmd_i dscl -f "$db" localhost -create "$record" RecordName "$username" || { rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"; msg_err "$(t USER_CREATE_FAILED)"; return 1; }
  run_cmd_i dscl -f "$db" localhost -create "$record" UniqueID "$uid" || { rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"; msg_err "$(t USER_CREATE_FAILED)"; return 1; }
  run_cmd_i dscl -f "$db" localhost -create "$record" PrimaryGroupID 20 || { rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"; msg_err "$(t USER_CREATE_FAILED)"; return 1; }
  run_cmd_i dscl -f "$db" localhost -create "$record" NFSHomeDirectory "/Users/$username" || { rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"; msg_err "$(t USER_CREATE_FAILED)"; return 1; }
  run_cmd_i dscl -f "$db" localhost -create "$record" GeneratedUID "$generated_uid" || { rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"; msg_err "$(t USER_CREATE_FAILED)"; return 1; }
  run_cmd_i dscl -f "$db" localhost -create "$record" Picture "/Library/User Pictures/Flowers/Lotus.heic" || true
  run_cmd_i dscl -f "$db" localhost -create "$record" dsAttrTypeNative:unlockOptions 0 || true
  run_cmd_i dscl -f "$db" localhost -append "$record" AuthenticationAuthority ";ShadowHash;" || {
    rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"
    msg_err "$(t USER_AUTH_FAILED)"
    return 1
  }
  if [ "$DRY_RUN" != "1" ]; then
    auth_output=$(dscl -f "$db" localhost -read "$record" AuthenticationAuthority 2>/dev/null)
    case "$auth_output" in
      *ShadowHash*) ;;
      *)
        rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"
        msg_err "$(t USER_AUTH_FAILED)"
        return 1
        ;;
    esac
  fi
  run_cmd_i dscl -f "$db" localhost -passwd "$record" "$password_arg" || {
    run_cmd_i dscl -f "$db" localhost -delete "$record" || true
    msg_err "$(t USER_CREATE_FAILED)"
    return 1
  }
  run_cmd_i dscl -f "$db" localhost -append /Local/Default/Groups/admin GroupMembership "$username" || {
    rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"
    msg_err "$(t USER_CREATE_FAILED)"
    return 1
  }
  apply_legacy_admin_attributes "$db" "$record" "$username"

  template=""
  for template in \
    "$(path_under_root "$SYSTEM_ROOT" "System/Library/User Template/zh_CN.lproj")" \
    "$(path_under_root "$SYSTEM_ROOT" "System/Library/User Template/English.lproj")"; do
    if [ -d "$template" ] && command_exists ditto; then
      run_cmd_i ditto -rsrc "$template" "$home" || true
      break
    fi
    template=""
  done
  if { [ "$DRY_RUN" = "1" ] && [ -z "$template" ]; } || { [ "$DRY_RUN" != "1" ] && [ ! -d "$home" ]; }; then
    run_cmd_i mkdir -p "$home" || {
      rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"
      msg_err "$(t USER_CREATE_FAILED)"
      return 1
    }
  fi
  if [ "$DRY_RUN" != "1" ]; then
    home_created=1
  fi
  template=$(path_under_root "$SYSTEM_ROOT" "System/Library/User Template/Non_localized")
  if [ -d "$template" ] && command_exists ditto; then
    run_cmd_i ditto -rsrc "$template" "$home" || true
  fi
  run_cmd_i chown -R "$uid:staff" "$home" || {
    rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"
    msg_err "$(t USER_CREATE_FAILED)"
    return 1
  }
  run_cmd_i chmod 0755 "$home" || {
    rollback_admin_user "$db" "$record" "$username" "$home" "$home_created"
    msg_err "$(t USER_CREATE_FAILED)"
    return 1
  }
  safe_touch "$(path_under_root "$TARGET_ROOT" "var/db/.AppleSetupDone")" || status=1
  if [ "$status" -eq 0 ]; then
    msg_ok "$(t USER_CREATED): $username (UID $uid)"
  else
    msg_err "$(t PARTIAL_DONE)"
  fi
  admin_password=""
  password_arg=""
  return "$status"
}

open_reset_password() {
  if [ "$RUN_MODE" != "recovery" ]; then
    msg_err "$(t RECOVERY_ONLY)"
    return 1
  fi
  command_exists resetpassword || { msg_err "$(t PASSWORD_TOOL_MISSING)"; return 1; }
  run_cmd_i resetpassword
}

set_sip() {
  local action="$1"
  command_exists csrutil || { msg_err "$(t SIP_TOOL_MISSING)"; return 1; }
  if [ "$RUN_MODE" != "recovery" ]; then
    msg_err "$(t RECOVERY_ONLY)"
    return 1
  fi
  run_cmd_v csrutil "$action"
}

clean_wifi_data() {
  local status=0
  if [ "$RUN_MODE" != "recovery" ]; then
    msg_err "$(t RECOVERY_ONLY)"
    return 1
  fi
  confirm_destructive || { msg_info "$(t CANCELLED)"; return 1; }
  safe_remove "$LIBRARY_PATH/Keychains/apsd.keychain" || status=1
  safe_remove "$LIBRARY_PATH/Preferences/com.apple.wifi.known-networks.plist" || status=1
  safe_remove "$LIBRARY_PATH/Preferences/SystemConfiguration/com.apple.airport.preferences.plist" || status=1
  if [ "$status" -eq 0 ]; then
    msg_ok "$(t DONE)"
  else
    msg_err "$(t PARTIAL_DONE)"
  fi
  return "$status"
}

select_regular_user() {
  local users=()
  local user=""
  while IFS= read -r user; do
    [ -n "$user" ] && users[${#users[@]}]="$user"
  done <<EOF
$(list_regular_users)
EOF
  if [ "${#users[@]}" -eq 0 ]; then
    msg_err "$(t NO_USER)"
    return 1
  elif [ "${#users[@]}" -eq 1 ]; then
    CURRENT_USER="${users[0]}"
  else
    choose_number "$(t SELECT_USER)" "${users[@]}" || return 1
    CURRENT_USER="${users[$SELECTED_INDEX]}"
  fi
}

change_root_password() {
  local db=""
  if [ "$RUN_MODE" != "recovery" ]; then
    msg_err "$(t RECOVERY_ONLY)"
    return 1
  fi
  db=$(path_under_root "$TARGET_ROOT" "private/var/db/dslocal/nodes/Default")
  command_exists dscl || { msg_err "$(t COMMAND_MISSING): dscl"; return 1; }
  msg_info "$(t ROOT_PASSWORD_PROMPT)"
  run_cmd_v dscl -f "$db" localhost -passwd /Local/Default/Users/root
}

disable_root_user() {
  local password=""
  local status=1
  if [ "$RUN_MODE" != "normal" ]; then
    msg_err "$(t NORMAL_ONLY)"
    return 1
  fi
  command_exists dsenableroot || { msg_err "$(t COMMAND_MISSING): dsenableroot"; return 1; }
  select_regular_user || return 1
  password="$SESSION_PASSWORD"
  if [ "$DRY_RUN" = "1" ]; then
    password="[REDACTED]"
  elif [ -z "$password" ]; then
    msg_err "$(t SUDO_PASSWORD_EMPTY)"
    return 1
  fi
  run_cmd_i dsenableroot -d -u "$CURRENT_USER" -p "$password"
  status=$?
  password=""
  [ "$status" -eq 0 ] && msg_ok "$(t ROOT_DISABLED)"
  return "$status"
}

clean_hosts_menu() {
  if ! confirm_destructive; then
    msg_info "$(t CANCELLED)"
    return 1
  fi
  msg_info "$(t HOSTS_UPDATING): $(path_under_root "$TARGET_ROOT" "etc/hosts")"
  if update_hosts clean; then
    [ "$DRY_RUN" = "1" ] || msg_ok "$(t HOSTS_UPDATED)"
    return 0
  fi
  return 1
}

main_menu() {
  local action=""
  local count=0
  local actions=()
  local options=()

  while :; do
    count=0
    actions=()
    options=()
    actions[count]="bypass"
    options[count]="$(t BYPASS_MDM)"
    count=$((count + 1))
    if [ "$RUN_MODE" = "recovery" ]; then
      actions[count]="create_user"
      options[count]="$(t CREATE_USER)"
      count=$((count + 1))
      actions[count]="reset_password"
      options[count]="$(t RESET_PASSWORD)"
      count=$((count + 1))
      actions[count]="disable_sip"
      options[count]="$(t DISABLE_SIP)"
      count=$((count + 1))
      actions[count]="enable_sip"
      options[count]="$(t ENABLE_SIP)"
      count=$((count + 1))
    fi
    actions[count]="clean_hosts"
    options[count]="$(t CLEAN_HOSTS)"
    count=$((count + 1))
    if [ "$RUN_MODE" = "recovery" ]; then
      actions[count]="clean_wifi"
      options[count]="$(t CLEAN_WIFI)"
      count=$((count + 1))
      actions[count]="change_root_password"
      options[count]="$(t CHANGE_ROOT_PASSWORD)"
      count=$((count + 1))
    fi
    if [ "$RUN_MODE" = "normal" ]; then
      actions[count]="disable_root"
      options[count]="$(t DISABLE_ROOT)"
      count=$((count + 1))
    fi
    actions[count]="exit"
    options[count]="$(t EXIT)"

    choose_number "$(t MAIN_MENU)" "${options[@]}" || return 1
    printf '\n' >&2
    action="${actions[$SELECTED_INDEX]}"
    case "$action" in
      bypass)
        bypass_mdm
        return 0
        ;;
      create_user) create_admin_user ;;
      reset_password) open_reset_password ;;
      disable_sip) set_sip disable ;;
      enable_sip) set_sip enable ;;
      clean_hosts) clean_hosts_menu ;;
      clean_wifi) clean_wifi_data ;;
      change_root_password) change_root_password ;;
      disable_root) disable_root_user ;;
      exit) return 0 ;;
    esac
  done
}

main() {
  read_inherited_password
  printf '\033[H\033[2J'
  select_language
  load_language_pack
  confirm_legal_notice || exit 1
  send_legal_notice_ping
  detect_environment
  if [ "$RUN_MODE" = "normal" ]; then
    msg_info "$(t MODE_NORMAL)"
  else
    msg_info "$(t MODE_RECOVERY)"
  fi
  [ "$DRY_RUN" = "1" ] && msg_info "$(t DRY_RUN_NOTICE)"
  ensure_root "$@"
  select_target_volume
  main_menu
}

if [ "${MDM_CLI_SOURCE_ONLY:-0}" != "1" ]; then
  main "$@"
fi
