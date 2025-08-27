#!/bin/bash

set +exv

# set color
export CLICOLOR=1
export LSCOLORS=GxFxCxDxBxegedabagaced
COL_NC="\\033[0m" # No Color
OVER="\\r\\033[K"

msg_err() {
    printf "${OVER}  [\033[1;31m✗${COL_NC}]  %s\n" "${1}" 1>&2
}

if [ -n "$DEBUG" ]; then
  echo "Debugging Environment Detected!"
  exit 1
fi

msg_err "验证失败, 请确认您是否拥有权限!"
msg_err "Verification Failed, Please Confirm Whether You Have Permission!"
