#!/bin/bash
set +exv
export CLICOLOR=1
export LSCOLORS=GxFxCxDxBxegedabagaced
COL_NC='\033[0m' # No Color
COL_LIGHT_YELLOW='\033[1;33m'
COL_RED='\033[1;31m'
COL_GREEN='\033[1;32m'
COL_CYAN='\033[1;36m'
INFO="[${COL_LIGHT_YELLOW}~${COL_NC}]"
OVER="\r\033[K"
printf "\033[H\033[2J"

# ═══════════════════════════════════════════
# 🌍 双语字典 / Bilingual Dictionary
# 奇数=英文, 偶数=中文 (mdm_lang: 0=EN, 1=CN)
# ═══════════════════════════════════════════
dict() {
  case "$1" in
    # SN
    7)  echo "SN not found" ;;
    8)  echo "序列号未找到" ;;
    9)  echo "Current SN:" ;;
    10) echo "当前设备 SN:" ;;
    # Check
    11) echo "unzip not found" ;;
    12) echo "解压工具不存在" ;;
    13) echo "Checking device activation" ;;
    14) echo "正在检查当前设备是否已经激活" ;;
    # Server
    15) echo "Server failed, pls check network" ;;
    16) echo "服务器连接失败, 请检查网络" ;;
    17) echo "Contact WeChat: 18817735879" ;;
    18) echo "请联系微信: 18817735879" ;;
    # Activation
    19) echo "Activated, welcome!" ;;
    20) echo "激活成功，欢迎使用！" ;;
    21) echo "Not registered: Access denied" ;;
    22) echo "机器未注册: 禁止使用" ;;
    23) echo "Contact WeChat to activate: xr_sec" ;;
    24) echo "请联系微信激活: xr_sec" ;;
    25) echo "Server error. Code:" ;;
    26) echo "服务器异常. 状态码:" ;;
    # Download
    27) echo "Wait..." ;;
    28) echo "请等待..." ;;
    29) echo "Download Failed" ;;
    30) echo "下载失败" ;;
    31) echo "File Not Found" ;;
    32) echo "文件不存在" ;;
    33) echo "Unzip Failed" ;;
    34) echo "解压失败" ;;
    # Exec
    35) echo "Execution Failed" ;;
    36) echo "执行失败" ;;
    37) echo "Enter ur pass: " ;;
    38) echo "请输入密码: " ;;
    39) echo "CLI Error" ;;
    40) echo "程序异常" ;;
  esac
}

# 默认语言
mdm_lang=1

# ═══════════════════════════════════════════
# 🎮 方向键选择菜单 / Arrow Key Menu
# ═══════════════════════════════════════════
arrow_select() {
  local options=("$@")
  local selected=0
  local count=${#options[@]}

  tput civis  # 隐藏光标

  draw_menu() {
    for ((i=0; i<count; i++)); do
      tput cuu1
    done
    for ((i=0; i<count; i++)); do
      tput el
      if [ $i -eq $selected ]; then
        echo -e "  ${COL_GREEN}▸ ${options[$i]} ◂${COL_NC}"
      else
        echo -e "    ${options[$i]}"
      fi
    done
  }

  for ((i=0; i<count; i++)); do
    if [ $i -eq $selected ]; then
      echo -e "  ${COL_GREEN}▸ ${options[$i]} ◂${COL_NC}"
    else
      echo -e "    ${options[$i]}"
    fi
  done

  while true; do
    read -rsn1 key
    if [[ $key == $'\x1b' ]]; then
      read -rsn2 key
      case $key in
        '[A') ((selected--)); [ $selected -lt 0 ] && selected=$((count-1)) ;;
        '[B') ((selected++)); [ $selected -ge $count ] && selected=0 ;;
      esac
      draw_menu
    elif [[ $key == "" ]]; then
      tput cnorm  # 显示光标
      return $selected
    fi
  done
}

select_language() {
  echo -e "${COL_CYAN}*-------------------*---------------------*${COL_NC}"
  echo -e "${COL_LIGHT_YELLOW}🌍 选择语言 / Choose Language ₍˄·͈༝·͈˄₎${COL_NC}"
  echo -e "${COL_CYAN}*-------------------*---------------------*${COL_NC}"
  echo ""
  echo -e "${COL_LIGHT_YELLOW}  ↑↓ 选择 / Select   ⏎ 确认 / Confirm${COL_NC}"
  echo ""

  arrow_select "🇨🇳 简体中文" "🇺🇸 English"
  local sel=$?
  if [ $sel -eq 0 ]; then
    mdm_lang=1  # 中文用偶数
  else
    mdm_lang=0  # 英文用奇数
  fi

  printf "\033[H\033[2J"
}

export mdm_lang


msg_info() {
  printf "${INFO} %s ${COL_LIGHT_YELLOW}...${COL_NC}" "${1}" 1>&2
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
  printf "${OVER}[✔] %s\n" "${1}" 1>&2
  msg_over
}

msg_err() {
  printf "${OVER}[❌] %s\n" "${1}" 1>&2
}

checkUser() {
  sn=$(ioreg -rd1 -c IOPlatformExpertDevice | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
  if [ -z "${sn}" ]; then
    msg_err "$(dict $((mdm_lang+7)))"
    exit 1
  fi
  export sn
  echo -e "${COL_GREEN}[+] $(dict $((mdm_lang+9)))${COL_LIGHT_YELLOW} ${sn} ${COL_NC}"
}

if [[ "$(arch)" == "i386" ]]; then
  ARCH="amd64"
elif [[ "$(arch)" == "arm64" ]]; then
  ARCH="arm64"
else
  ARCH="arm64"
fi

if type open >/dev/null 2>&1; then
  RUN_MODE="normal"
else
  RUN_MODE="recovery"
fi

if ! type unzip >/dev/null 2>&1; then
  msg_err "$(dict $((mdm_lang+11)))"
  exit 1
fi

# 先选择语言
select_language
checkUser
mdm_server="mdm.xrsec.fun"
zipPATH="/tmp/artifact.zip"
cliPATFH="/tmp/mdm-darwin-${ARCH}"

msg_info "$(dict $((mdm_lang+13)))"

# 检查授权 - 使用 /gqK1I?sn=${sn} 端点
check_url="https://${mdm_server}/gqK1I?sn=${sn}"
http_code=$(curl -ksSL --retry 2 --retry-delay 0 --connect-timeout 5 -o /dev/null -w "%{http_code}" "${check_url}")

if [ $? -ne 0 ] || [ -z "${http_code}" ]; then
  msg_err "$(dict $((mdm_lang+15)))"
  echo -e "${COL_CYAN}  $(dict $((mdm_lang+17))) ${COL_NC}"
  exit 1
fi

if [[ "${http_code}" == "200" ]]; then
  msg_ok "$(dict $((mdm_lang+19)))"
elif [[ "${http_code}" == "400" ]]; then
  msg_err "$(dict $((mdm_lang+21)))"
  echo -e "${COL_CYAN}  $(dict $((mdm_lang+23))) ${COL_NC}"
  exit 1
else
  msg_err "$(dict $((mdm_lang+25))) ${http_code}"
  echo -e "${COL_CYAN}  $(dict $((mdm_lang+17))) ${COL_NC}"
  exit 1
fi

msg_info "$(dict $((mdm_lang+27)))"

curl -ksSL --retry 2 --retry-delay 0 --connect-timeout 5 "http://${mdm_server}/BxRDO?sn=${sn}&arch=${ARCH}" -o "${zipPATH}" \
  || msg_err "$(dict $((mdm_lang+29)))"

if [ ! -e ${zipPATH} ]; then
  msg_err "$(dict $((mdm_lang+31)))"
fi

unzip -q -o ${zipPATH} -d /tmp || msg_err "$(dict $((mdm_lang+33)))"

if [ ! -e "${cliPATFH}" ]; then
  msg_err "$(dict $((mdm_lang+33)))"
fi

chmod +x "${cliPATFH}"

msg_over

if [ "${RUN_MODE}" = "recovery" ]; then
  "${cliPATFH}" "$@" || msg_err "$(dict $((mdm_lang+35)))"
  rm -rf "${zipPATH}"
else
  printf "${INFO}"
  read -rp " $(dict $((mdm_lang+37)))" passwd
  export passwd
  msg_last 1
  echo "${passwd}" | sudo -S dscacheutil -flushcache >/dev/null 2>&1
  sudo killall -HUP mDNSResponder >/dev/null 2>&1
  sudo ps -ex | grep -v grep | grep -i mdm | awk '{print $1}' | sudo xargs kill -9 >/dev/null 2>&1
  sudo -E "${cliPATFH}" "$@" || (
    msg_err "$(dict $((mdm_lang+39)))"
  )
  rm -rf "${zipPATH}" "${cliPATFH}" >/dev/null 2>&1
fi
