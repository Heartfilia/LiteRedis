#!/bin/zsh

set -euo pipefail

PAGE_URL_NEW="x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension"
PAGE_URL_OLD="x-apple.systempreferences:com.apple.preference.security?Privacy"

if ! open "$PAGE_URL_NEW" 2>/dev/null; then
  open "$PAGE_URL_OLD"
fi

osascript <<'APPLESCRIPT'
display dialog "已为你打开“系统设置 -> 隐私与安全性”。\n\n如果 LiteRedis 被拦截，请在该页面底部点击“仍要打开”。" buttons {"知道了"} default button "知道了"
APPLESCRIPT
