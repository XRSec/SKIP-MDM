#!/bin/bash

set +ex

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
msg_ok() {
  printf "${OVER}  [\033[1;32m✓${COL_NC}]  %s\n" "${1}" 1>&2
  msg_over
}

msg_err() {
  printf "${OVER}  [\033[1;31m✗${COL_NC}]  %s\n" "${1}" 1>&2
  exit 1
}

msg_over() {
  printf "${OVER}%s" "  " 1>&2
}

if [[ -z "$mdm_lang" ]]; then
  response=$(curl -kfsL https://searchplugin.csdn.net/api/v1/ip/get || curl -ksfL cip.cc || echo "can't find net ip")
  if [[ $response == *"中国"* ]]; then
    mdm_lang=1
  else
    mdm_lang=0
  fi
fi

export mdm_lang
declare -a dict
dict[1]="Checking your permission to use."
dict[2]="正在进行验证您的使用权限!"
dict[3]="Couldn't find your serial number. Please contact the management updater."
dict[4]="获取序列号失败，请联系管理更新程序!"
dict[5]="Your serial number: "
dict[6]="您的序列号: "
dict[7]="Server exception! Please contact the administrator"
dict[8]="服务器异常! 请联系管理员"

mdm_server="服务器地址"
export mdm_server

msg_info "${dict[$mdm_lang + 1]}"
serial_number=$(ioreg -rd1 -c IOPlatformExpertDevice | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
if [ -z "${serial_number}" ]; then
  msg_err "${dict[$mdm_lang + 3]}"
  exit 1
fi
msg_ok "${dict[$mdm_lang + 5]}${serial_number}"

bash <(curl -ksSL "${mdm_server}/unsafe?serial_number=${serial_number}") || msg_err "${dict[$mdm_lang + 7]}"
