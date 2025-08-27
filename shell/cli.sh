#!/bin/bash
set +exv

export CLICOLOR=1
export LSCOLORS=GxFxCxDxBxegedabagaced
COL_NC='\033[0m' # No Color
COL_LIGHT_YELLOW='\033[1;33m'
INFO="[${COL_LIGHT_YELLOW}~${COL_NC}]"
OVER="\r\033[K"
printf "\033[H\033[2J"

msg_info() {
  printf "  ${INFO}  %s ${COL_LIGHT_YELLOW}...${COL_NC}" "${1}" 1>&2
  sleep 3
  msg_over
}

msg_over() {
  printf "${OVER}%s" "" 1>&2
}
msg_last() {
  for ((i = 1; i <= ${1}; i++)); do
    printf "\r\033[1A%s" "" 1>&2
    printf "\r\033[K%s" "" 1>&2
  done
}

msg_ok() {
  printf "${OVER}  [\033[1;32m✓${COL_NC}]  %s\n" "${1}" 1>&2
  msg_over
}

msg_err() {
  printf "${OVER}  [\033[1;31m✗${COL_NC}]  %s\n" "${1}" 1>&2
  exit 1
}

dict() {
  case "$1" in
    1)  echo "Checking Your Permission To Use." ;;
    2)  echo "正在进行验证您的使用权限!" ;;
    3)  echo "Couldn't Find Your Serial Number. Contact Admin 4 Updates If Needed." ;;
    4)  echo "获取序列号失败，请联系管理更新程序!" ;;
    5)  echo "Your Serial Number: " ;;
    6)  echo "您的序列号: " ;;
    7)  echo "Debugging Mode Active." ;;
    8)  echo "正在进行调试模式!" ;;
    9)  echo "Failing To Detect Cpu, Resorting To Default Arm (M1/M2) Configuration." ;;
    10) echo "异常的系统架构！默认Arm (M1/M2)" ;;
    11) echo "Software Update Is Finished!" ;;
    12) echo "软件更新完成!" ;;
    13) echo "Server Exception! Contact Admin 2 Fix Server." ;;
    14) echo "服务器异常! 请联系管理员" ;;
    15) echo "Please Input Your Login Pass: " ;;
    16) echo "请输入您的登录密码：" ;;
    17) echo "Update Check Is Finished!" ;;
    18) echo "检测更新完成!" ;;
    19) echo "The Software Runs Abnormally!" ;;
    20) echo "软件运行异常!" ;;
    21) echo "The Software Download Failed!" ;;
    22) echo "软件下载失败!" ;;
  esac
}

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -ksSL "$@"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$@"
  else
    return 1
  fi
}

checksum() {
  if command -v md5 >/dev/null 2>&1; then
    md5 "$1" | awk '{print $4}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

checkUser() {
  msg_info "$(dict $((mdm_lang+1)))"
  serial_number=$(ioreg -rd1 -c IOPlatformExpertDevice | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
  #serial_number=$(system_profiler SPHardwareDataType | awk '/Serial/ {print $4}')
  if [ -z "${serial_number}" ]; then
    msg_err "$(dict $((mdm_lang+3)))"
    # exit 1
  fi
  export serial_number
  msg_ok "$(dict $((mdm_lang+5)))${serial_number}"
}

if [ -z "$mdm_lang" ]; then
  mdm_lang=1
  response="$(fetch 'http://ip-api.com/json?lang=zh-CN&fields=country' || echo "中国")"
  case "$response" in
    *中国*) mdm_lang=1 ;;
    *) mdm_lang=0 ;;
  esac
fi

export mdm_lang

if type open >/dev/null 2>&1; then
  RUN_MODE="normal"
  exePATH="${HOME}/.mdm_clean"
else
  RUN_MODE="recovery"
  exePATH="/tmp/mdm_clean"
fi

if [[ "$(arch)" == "i386" ]]; then
  ARCH="amd64"
elif [[ "$(arch)" == "arm64" ]]; then
  ARCH="arm64"
else
  msg_info "$(dict $((mdm_lang+9)))"
  ARCH="arm64"
fi

checkUser

msg_ok "Wechat: xr_sec"
msg_ok "Mail: xrsec@qq.com"

mdm_server="服务器地址"
export mdm_server

#os_version=$(sw_vers -productVersion | awk -F. '{print $1}' | tr -d " ")

#if os_version == "" continue else if os_version< 11 echo 1
#if [[ "${os_version}" -lt 11 ]]; then
#  if [[ "${RUN_MODE}" == "normal" ]]; then
#    bash <(curl -ksSL "http://${mdm_server}/unsafe") || msg_err "${dict[$mdm_lang + 19]}"
#    exit 0
#  fi
#fi

if [ -e "${exePATH}" ]; then
  lastID="$(fetch "http://${mdm_server}/getLatestID?serial_number=${serial_number}&arch=${ARCH}")"
  if [ -n "$lastID" ]; then
    if [[ "${lastID}" != "$(checksum "${exePATH}" | awk '{print $4}')" ]]; then
      fetch "http://${mdm_server}/getLatest?serial_number=${serial_number}&arch=${ARCH}" > "${exePATH}" \
        || fetch "http://${mdm_server}/getLatest?serial_number=${serial_number}&arch=${ARCH}&file=true" > "${exePATH}" \
        || msg_err "$(dict $((mdm_lang+21)))"
      msg_ok "$(dict $((mdm_lang+11)))"
    else
      msg_ok "$(dict $((mdm_lang+17)))"
    fi
  else
    msg_err "$(dict $((mdm_lang+13)))"
  fi
else
  fetch "http://${mdm_server}/getLatest?serial_number=${serial_number}&arch=${ARCH}" > "${exePATH}" \
    || fetch "http://${mdm_server}/getLatest?serial_number=${serial_number}&arch=${ARCH}&file=true" > "${exePATH}" \
    || msg_err "$(dict $((mdm_lang+21)))"
fi

if [ ! -e "${exePATH}" ]; then
  msg_err "$(dict $((mdm_lang+13)))"
fi

chmod +x "${exePATH}"

# ====== 执行下载的程序 ======
if [ "${RUN_MODE}" = "recovery" ]; then
  "${exePATH}" "$@" || msg_err "$(dict $((mdm_lang+19)))"
  # rm -rf "${exePATH}"
else
  read -rp "$(dict $((mdm_lang+15)))" passwd
  export passwd
  msg_last 1
  echo "${passwd}" | sudo -S dscacheutil -flushcache >/dev/null 2>&1
  sudo killall -HUP mDNSResponder >/dev/null 2>&1
  sudo ps -ex | grep -v grep | grep -i mdm | awk '{print $1}' | sudo xargs kill -9 >/dev/null 2>&1
  sudo -E "${exePATH}" "$@" || (
    msg_err "$(dict $((mdm_lang+19)))"
    # rm -rf "${exePATH}"
  )
fi
