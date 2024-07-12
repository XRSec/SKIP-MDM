#!/bin/bash

set +exv

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

if [ -n "$DEBUG" ]; then
  echo "Debugging environment detected!"
  exit 1
fi

findOSPATH() {
  IFSSet
  OSPATH=$(find -L /Volumes -iname Users -type d -maxdepth 2 -follow 2>&1 | grep -vE "^/Volumes/[^/]*(数据|Data|System|private)")
#  OSPATH=$(find -L /Volumes -iname Users -type d -maxdepth 2 -follow 2>&1 | grep -vE "\b数据|\bData|\bSystem|\n|private")

  if [ -z "${OSPATH}" ]; then
    msg_err "${dict[$mdm_lang + 1]}"
  fi

  if [ "$(echo "${OSPATH}" | wc -l)" -gt 1 ]; then
    msg_info "${dict[$mdm_lang + 3]}"
    msg_over
    # shellcheck disable=SC2039
    select variable in ${OSPATH}; do
      if [ -n "${variable}" ]; then
        tempPath="${variable%/*}"
        tempPath2="${tempPath##*/}"
        printf "  "
        msg_info "${dict[$mdm_lang + 5]} ${tempPath2} ${dict[$mdm_lang + 7]}"
        msg_over
        OSPATH="${tempPath}"
        IFSRestore
        break
      else
        msg_info "${dict[$mdm_lang + 9]}"
      fi
    done
  else
    OSPATH="${OSPATH%/*}"
  fi

  IFSRestore
}

setHosts() {
  msg_info "${dict[$mdm_lang + 11]}"
  msg_over
  IFSSet
  Hosts="${OSPATH}/etc/hosts"

  if [ ! -e "${Hosts}" ]; then
    msg_ok "${dict[$mdm_lang + 13]}"
    msg_over
  fi
  # https://gist.github.com/henrik242/65d26a7deca30bdb9828e183809690bd
  for file in ${Hosts}; do
    if [[ "$(awk 'END {print}' "${file}")" != "" ]]; then
      echo "" >>"${file}" 2>/dev/null
    fi

    if [[ "$(grep -c "^0.0.0.0 iprofiles.apple.com" "${file}")" -eq '0' ]]; then
      echo "0.0.0.0 iprofiles.apple.com" >>"${file}" 2>/dev/null
    fi

    if [[ "$(grep -c "^0.0.0.0 mdmenrollment.apple.com" "${file}")" -eq '0' ]]; then
      echo "0.0.0.0 mdmenrollment.apple.com" >>"${file}" 2>/dev/null
    fi

    if [[ "$(grep -c "^0.0.0.0 deviceenrollment.apple.com" "${file}")" -eq '0' ]]; then
      echo "0.0.0.0 deviceenrollment.apple.com" >>"${file}" 2>/dev/null
    fi
  done

  msg_ok "${dict[$mdm_lang + 15]}"
  msg_over
  IFSRestore
}

cleanMdm() {
  if [ ! -d "${MDMPath}" ]; then
    msg_info "${dict[$mdm_lang + 17]}"
    return
  fi

  msg_info "${dict[$mdm_lang + 19]}"
  msg_over
  nvram -c >/dev/null 2>&1
  rm -rfv "${MDMPath}/Settings/.profilesAreInstalled" >/dev/null 2>&1
  rm -rfv "${MDMPath}/Store" >/dev/null 2>&1
  rm -rfv "${MDMPath}/Settings" >/dev/null 2>&1
  rm -rfv "${LibraryPath}/Keychains/apsd.keychain" >/dev/null 2>&1
  rm -rfv "${LibraryPath}/Preferences/com.apple.wifi.known-networks.plist" >/dev/null 2>&1
  rm -rfv "${LibraryPath}/Preferences/SystemConfiguration/com.apple.airport.preferences.plist" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.profilesAreInstalled" >/dev/null 2>&1
  touch "${MDMPath}/Store" >/dev/null 2>&1
  mkdir "${MDMPath}/Settings" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigRecordNotFound" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigProfileInstalled" >/dev/null 2>&1 # https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4591775#gistcomment-4591775
  touch "${MDMPath}/Settings/.cloudConfigNoActivationRecord" >/dev/null 2>&1
  touch "${MDMPath}/Settings/.cloudConfigUserSkippedEnrollment" >/dev/null 2>&1

  msg_ok "${dict[$mdm_lang + 21]}"
  msg_over
}

checkUser() {
  if [ "$(find "${OSPATH}/Users" -type d -maxdepth 1 ! -name "Shared" ! -name "/" 2>/dev/null | wc -l)" -gt 1 ]; then
    return
  fi
  if [ "$(sw_vers -productVersion | awk -F. '{print $1}')" -le 12 ] ; then
    return
  fi
  dscl_path="${OSPATH}/private/var/db/dslocal/nodes/Default"
  maxid=$(dscl -f "$dscl_path" localhost -list "/Local/Default/Users" UniqueID | awk 'BEGIN { max = 500; } { if ($2 > max) max = $2; } END { print max + 1; }')
  account_id=$((maxid + 1))
  # account_id="${RANDOM}"
  username="mac_${account_id}"
  passwd="123456"
  msg_ok "Name: ${username}"
  msg_ok "Pass: ${passwd}"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "UserShell" "/bin/zsh"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "RealName" "Mac"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "UniqueID" "${account_id}"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "PrimaryGroupID" "20"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "AuthenticationHint" "by(vx): xr_sec passwd: $passwd"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "Picture" "/Library/User Pictures/Flowers/Lotus.heic"
  ditto -rsrc "${OSPATH}/System/Library/User Template/zh_CN.lproj" "${OSPATH}/Users/$username"
  ditto -rsrc "${OSPATH}/System/Library/User Template/Non_localized" "${OSPATH}/Users/$username"
  chown -R "$account_id:staff" "${OSPATH}/Users/$username"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "NFSHomeDirectory" "/Users/$username"
  dscl -f "$dscl_path" localhost -passwd "/Local/Default/Users/$username" "$passwd"
  dscl -f "$dscl_path" localhost -append "/Local/Default/Groups/admin" "GroupMembership" "$username"
  # dscl -f "$dscl_path" localhost -append "/Local/Default/Users/$username" "AuthenticationAuthority" ";DisabledTags;SecureToken" # 傻逼谷歌卧槽尼玛
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "dsAttrTypeNative:_defaultLanguage" "zh_CN"
  dscl -f "$dscl_path" localhost -create "/Local/Default/Users/$username" "dsAttrTypeNative:_writers__defaultLanguage" "$username"
  touch "${OSPATH}/private/var/db/.AppleSetupDone"
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
dict[13]="Unable to Open Hosts file"
dict[14]="无法打开 Hosts 文件"
dict[15]="Blocked MDM/DEP!"
dict[16]="屏蔽 MDM/DEP 完成!"
dict[17]="Please exit the terminal, go to disk utility, expand all disks (arrow) to find ${OSPATH} - DATA, select mount, then exit disk utility and return to the terminal to rerun the program",
dict[18]="请退出终端, 前往磁盘工具, 将磁盘全部展开(箭头) 找到 ${OSPATH} - DATA , 选择装载, 接着退出磁盘工具回到终端重新运行程序"
dict[19]="Cleaning MDM/DEP."
dict[20]="正在清理 MDM/DEP."
dict[21]="Cleaned MDM/DEP!"
dict[22]="清理 MDM/DEP 完成!"
dict[23]="Rebooting Mac."
dict[24]="正在重启 Mac."
dict[25]="Temp User Please Delete"
dict[26]="临时用户 请及时删除"

if [[ -z "$mdm_lang" ]]; then
  response=$(curl -kfsL https://searchplugin.csdn.net/api/v1/ip/get || curl -ksfL cip.cc || echo "can't find net ip")
  if [[ $response == *"中国"* ]]; then
    mdm_lang=1
  else
    mdm_lang=0
  fi
fi

export mdm_lang
if type open >/dev/null 2>&1; then
  exit 0
fi
findOSPATH
MDMPath="${OSPATH}/var/db/ConfigurationProfiles"
LibraryPath="${OSPATH}/Library"
checkUser
setHosts
cleanMdm

msg_info "${dict[$mdm_lang + 23]}"
msg_over
reboot now
