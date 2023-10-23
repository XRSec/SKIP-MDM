#!/usr/bin/env bash

export CLICOLOR=1
export LSCOLORS=GxFxCxDxBxegedabagaced
COL_NC='\033[0m' # No Color
COL_LIGHT_YELLOW='\033[1;33m'
INFO="[${COL_LIGHT_YELLOW}~${COL_NC}]"
OVER="\\r\\033[K"

msg_info() {
  printf "${INFO}  %s ${COL_LIGHT_YELLOW}...${COL_NC}" "${1}" 1>&2
  sleep 3
}

msg_ok() {
  printf "${OVER}  [\033[1;32m✓${COL_NC}]  %s\n" "${1}" 1>&2
}

msg_err() {
  printf "${OVER}  [\033[1;31m✗${COL_NC}]  %s\n" "${1}" 1>&2
  exit 1
}
msg_over() {
  printf "${OVER}%s" "  " 1>&2
}

IFSSet() {
  ifsBak=$IFS
  IFS='
'
}

IFSRestore() {
  IFS=$ifsBak
}

findOSPATH() {
  IFSSet
  OSPATH=$(find -L /Volumes -iname Users -type d -maxdepth 2 -follow 2>&1 | grep -vE "\- Data|Data|System|\n|private|macOS Base System")

  if [ -z "${OSPATH}" ]; then
    msg_err "未找到系统盘."
  fi

  if [ "$(echo "${OSPATH}" | wc -l)" -gt 1 ]; then
    msg_info "找到多个系统盘, 常见的系统启动盘为: Macintosh HD, 请选择你的系统盘"
    msg_over
    # shellcheck disable=SC2039
    select variable in ${OSPATH}; do
      if [ -n "${variable}" ]; then
        tempPath="${variable%/*}"
        tempPath2="${tempPath##*/}"
        printf "  "
        msg_info "选择了 ${tempPath2} 目录"
        msg_over
        OSPATH="${tempPath}"
        IFSRestore
        break
      else
        msg_info "输入错误."
      fi
    done
  else
    OSPATH="${OSPATH%/*}"
    #    OSPATH="${OSPATH##*/}"
  fi

  IFSRestore
}

setHosts() {
  msg_info "正在屏蔽监管锁"
  msg_over
  IFSSet
  Hosts="${OSPATH}/etc/hosts"

  if [ -e "${Hosts}" ]; then
    msg_ok "当前是新机模式! 将跳过 HOSTS 屏蔽"
    msg_over
  fi
  # https://gist.github.com/henrik242/65d26a7deca30bdb9828e183809690bd
  for file in ${Hosts}; do
    if [[ "$(awk 'END {print}' "${file}")" != "" ]]; then
      tee -a "${file}" >/dev/null <<-EOF

EOF
    fi

    if [[ "$(grep -c "^0.0.0.0 iprofiles.apple.com" "${file}")" -eq '0' ]]; then
      tee -a "${file}" >/dev/null <<-EOF
0.0.0.0 iprofiles.apple.com
EOF
    fi

    if [[ "$(grep -c "^0.0.0.0 mdmenrollment.apple.com" "${file}")" -eq '0' ]]; then
      tee -a "${file}" >/dev/null <<-EOF
0.0.0.0 mdmenrollment.apple.com
EOF
    fi

    if [[ "$(grep -c "^0.0.0.0 deviceenrollment.apple.com" "${file}")" -eq '0' ]]; then
      tee -a "${file}" >/dev/null <<-EOF
0.0.0.0 deviceenrollment.apple.com
EOF
    fi
  done

  msg_ok "屏蔽监管接口完成!"
  msg_over
  IFSRestore
}

cleanMdm() {
  ## 检查 监管软件概要文件夹
  if [ "$(basename "${MDMPath}")" != "ConfigurationProfiles" ]; then
    msg_err "未找到监管软件概要文件夹, 请联系管理员修复脚本"
  fi

  msg_info "正在清理"
  msg_over
  nvram -c
  rm -rfv  "${MDMPath}/.profilesAreInstalled" >/dev/null 2>&1
  rm -rfv  "${MDMPath}/Store">/dev/null 2>&1
  rm -rfv  "${MDMPath}/Settings">/dev/null 2>&1
  rm -rfv  "${LibraryPath}/Keychains/apsd.keychain">/dev/null 2>&1
  rm -rfv  "${LibraryPath}/Preferences/com.apple.wifi.known-networks.plist">/dev/null 2>&1
  rm -rfv  "${LibraryPath}/Preferences/SystemConfiguration/com.apple.airport.preferences.plist">/dev/null 2>&1
  touch "${MDMPath}/.profilesAreInstalled" >/dev/null 2>&1
  touch "${MDMPath}/Store" >/dev/null 2>&1
  mkdir "${MDMPath}/Settings" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigRecordNotFound" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigProfileInstalled" >/dev/null 2>&1 # https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4591775#gistcomment-4591775
  touch "${MDMPath}/Settings/.cloudConfigNoActivationRecord" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigUserSkippedEnrollment" >/dev/null 2>&1

  msg_ok "清理监管完成"
  msg_over
}

if type open >/dev/null 2>&1; then
  exit 0
fi

findOSPATH

MDMPath="${OSPATH}/var/db/ConfigurationProfiles"
LibraryPath="${OSPATH}/Library"

setHosts

msg_ok "系统完整性检查完毕."
msg_over
cleanMdm
  msg_info "即将重启电脑."
  msg_over
reboot