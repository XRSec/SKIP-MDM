#!/bin/bash

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
    msg_err "${dict[$language + 1]}"
  fi

  if [ "$(echo "${OSPATH}" | wc -l)" -gt 1 ]; then
    msg_info "${dict[$language + 3]}"
    msg_over
    # shellcheck disable=SC2039
    select variable in ${OSPATH}; do
      if [ -n "${variable}" ]; then
        tempPath="${variable%/*}"
        tempPath2="${tempPath##*/}"
        printf "  "
        msg_info "${dict[$language + 5]} ${tempPath2} ${dict[$language + 7]}"
        msg_over
        OSPATH="${tempPath}"
        IFSRestore
        break
      else
        msg_info "${dict[$language + 9]}"
      fi
    done
  else
    OSPATH="${OSPATH%/*}"
    #    OSPATH="${OSPATH##*/}"
  fi

  IFSRestore
}

setHosts() {
  msg_info "${dict[$language + 11]}"
  msg_over
  IFSSet
  Hosts="${OSPATH}/etc/hosts"

  if [ -e "${Hosts}" ]; then
    msg_ok "${dict[$language + 13]}"
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

  msg_ok "${dict[$language + 15]}"
  msg_over
  IFSRestore
}

cleanMdm() {
  if [ "$(basename "${MDMPath}")" != "ConfigurationProfiles" ]; then
    msg_err "${dict[$language + 17]}"
  fi

  msg_info "${dict[$language + 19]}"
  msg_over
  nvram -c
  rm -rfv "${MDMPath}/.profilesAreInstalled" >/dev/null 2>&1
  rm -rfv "${MDMPath}/Store" >/dev/null 2>&1
  rm -rfv "${MDMPath}/Settings" >/dev/null 2>&1
  rm -rfv "${LibraryPath}/Keychains/apsd.keychain" >/dev/null 2>&1
  rm -rfv "${LibraryPath}/Preferences/com.apple.wifi.known-networks.plist" >/dev/null 2>&1
  rm -rfv "${LibraryPath}/Preferences/SystemConfiguration/com.apple.airport.preferences.plist" >/dev/null 2>&1
  touch "${MDMPath}/.profilesAreInstalled" >/dev/null 2>&1
  touch "${MDMPath}/Store" >/dev/null 2>&1
  mkdir "${MDMPath}/Settings" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigRecordNotFound" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigProfileInstalled" >/dev/null 2>&1 # https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4591775#gistcomment-4591775
  touch "${MDMPath}/Settings/.cloudConfigNoActivationRecord" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigUserSkippedEnrollment" >/dev/null 2>&1

  msg_ok "${dict[$language + 21]}"
  msg_over
}

declare -a dict
dict[1]="Can't Find Disk (Please use 『Disk Utility』 Mount Disk)."
dict[2]="找不到磁盘 (请使用『磁盘工具』挂载磁盘)。"
dict[3]="Choose U System Disk(Macintosh HD): "
dict[4]="选择系统盘 (Macintosh HD):"
dict[5]="Choose"
dict[6]="选择"
dict[7]="Disk"
dict[8]="磁盘"
dict[9]="Input Error."
dict[10]="输入错误."
dict[11]="Blocking MDM/DEP."
dict[12]="正在屏蔽 MDM/DEP."
dict[13]="New System, Pass Block!"
dict[14]="新系统, 跳过屏蔽"
dict[15]="Blocked MDM/DEP!"
dict[16]="屏蔽 MDM/DEP 完成!"
dict[17]="MDM/DEP Dir Error!"
dict[18]="MDM/DEP 目录错误!"
dict[19]="Cleaning MDM/DEP."
dict[20]="正在清理 MDM/DEP."
dict[21]="Cleaned MDM/DEP!"
dict[22]="清理 MDM/DEP 完成!"
dict[23]="Rebooting Mac."
dict[24]="正在重启 Mac."


response=$(curl -s http://cip.cc)
if [[ $response == *"中国"* ]]; then
  language=1
else
  language=0
fi

if type open >/dev/null 2>&1; then
  exit 0
fi

findOSPATH
MDMPath="${OSPATH}/var/db/ConfigurationProfiles"
LibraryPath="${OSPATH}/Library"
setHosts
cleanMdm
msg_info "${dict[$language + 23]}"
msg_over
reboot
