#!/bin/bash

# set -ex

# set color
export CLICOLOR=1
export LSCOLORS=GxFxCxDxBxegedabagaced
COL_NC='\033[0m' # No Color
COL_LIGHT_YELLOW='\033[1;33m'
INFO="[${COL_LIGHT_YELLOW}~${COL_NC}]"
OVER="\r\033[K"
printf "\033[H\033[2J" # 清理屏幕
# set msg
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

response=$(curl -s http://cip.cc)
if [[ $response == *"中国"* ]]; then
  language=1
else
  language=0
fi

declare -a dict
dict[1]="Checking your permission to use."
dict[2]="正在进行验证您的使用权限!"
dict[3]="Couldn't find your serial number. Please contact the management updater."
dict[4]="获取序列号失败，请联系管理更新程序!"
dict[5]="Your serial number: "
dict[6]="您的序列号: "
dict[7]="Debugging mode active."
dict[8]="正在进行调试模式!"
dict[9]="Failing to detect CPU, resorting to Default ARM (M1/M2) configuration."
dict[10]="异常的系统架构！默认ARM (M1/M2)"
dict[11]="Software update is finished!"
dict[12]="软件更新完成!"
dict[13]="Server exception! Please contact the administrator"
dict[14]="服务器异常! 请联系管理员"
dict[15]="Please input your login password: "
dict[16]="请输入您的登录密码："
dict[17]="Update check is finished!"
dict[18]="检测更新完成!"
dict[19]="The software runs abnormally!"
dict[20]="软件运行异常!"

checkUser() {
  msg_info "${dict[$language + 1]}"
  serial_number=$(ioreg -rd1 -c IOPlatformExpertDevice | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
  if [ -z "${serial_number}" ]; then
    msg_err "${dict[$language + 3]}"
    exit 1
    #serial_number=$(system_profiler SPHardwareDataType | awk '/Serial/ {print $4}')
  fi
  msg_ok "${dict[$language + 5]}${serial_number}"
}

if [[ "${mdm_debug}" == "true" ]]; then
  msg_info "${dict[$language + 7]}"
  set -ex
fi

# 检查当前运行环境
if type open >/dev/null 2>&1; then
  OSTYPE="recovery"
  exePATH="${HOME}/.mdm_clean"
else
  OSTYPE="normal"
  exePATH="/tmp/mdm_clean"
fi

if [[ "$(arch)" == "i386" ]]; then
  ARCH="amd64"
elif [[ "$(arch)" == "arm64" ]]; then
  ARCH="arm64"
else
  msg_info "${dict[$language + 9]}"
  ARCH="arm64"
fi

checkUser

jsons="$(curl -skL 'https://www.ssleye.com/ssltool/dns_check_hander' --data-raw 'domain=mdms.fun&dns=A')"

if echo "$jsons" | grep -q "\"error\": true"; then
  server_URL="$(echo "${jsons}" | awk -F'"' '/msg/{print $4}')"
else
  server_URL="mdms.fun"
fi

if [[ -e "${exePATH}" ]]; then
  lastID=$(curl -skL "http://${server_URL}/getLatestID?serial_number=${serial_number}&arch=${ARCH}")
  if [[ "${lastID}" != "" ]]; then
    if [[ "${lastID}" != "$(md5 ${exePATH} | awk '{print $4}')" ]]; then
      curl -skLo ${exePATH} "http://${server_URL}/getLatest?serial_number=${serial_number}&arch=${ARCH}" || curl -skLo ${exePATH} "http://${server_URL}/getLatest?serial_number=${serial_number}&arch=${ARCH}&file=true"
      msg_ok "${dict[$language + 11]}"
    else
      msg_ok "${dict[$language + 17]}"
    fi
  else
    msg_err "${dict[$language + 13]}"
  fi
else
  curl -skLo ${exePATH} "http://${server_URL}/getLatest?serial_number=${serial_number}&arch=${ARCH}" || curl -skLo ${exePATH} "http://${server_URL}/getLatest?serial_number=${serial_number}&arch=${ARCH}&file=true"
fi

if [[ ! -e "${exePATH}" ]]; then
  msg_err "${dict[$language + 13]}"
fi

chmod +x "${exePATH}"

if [[ "${OSTYPE}" == "normal" ]]; then
  "${exePATH}" -sn="${serial_number}" "$@" || (
    msg_err "${dict[$language + 19]}"
    rm -rf "${exePATH}"
  )
else
  read -p "${dict[$language + 15]}" -r passwd
  msg_last 1
  echo "${passwd}" | sudo -S dscacheutil -flushcache
  sudo killall -HUP mDNSResponder
  sudo ps -ex | grep -v grep | grep -i mdm | awk '{print $1}' | sudo xargs kill -9 >/dev/null 2>&1
  sudo -E "${exePATH}" -sn="${serial_number}" "$@" || (
    msg_err "${dict[$language + 19]}"
    rm -rf "${exePATH}"
  )
fi
