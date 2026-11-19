#!/bin/bash
# App Store screenshots from the simulators, with no tapping: a Debug build of the app runs the JavaScript it is
# launched with (`-mardoScript`, see WebScreen.swift) a few seconds after it starts. Needs the Debug simulator
# build (make ios-app) and the two simulators below booted. Results go to $OUT (default ~/Desktop/mardo-appstore-new).
#   ios/screenshots.sh            everything
#   ios/screenshots.sh phone      only the iPhone 6.9" set (1320x2868)
#   ios/screenshots.sh pad        only the iPad 13" set (2064x2752)
set -u
cd "$(dirname "$0")/.."
PHONE=${PHONE:-DF85B068-A739-4B07-9C1C-F2E6DDD5C1F4}   # iPhone 17 Pro Max
PAD=${PAD:-A21E5766-A7E1-4DAA-B37D-60657F157EB0}       # iPad Pro 13-inch (M5)
APP=${APP:-ios/build/Build/Products/Debug-iphonesimulator/Mardo.app}
BUNDLE=com.henryvandenbroek.mardo
OUT=${OUT:-$HOME/Desktop/mardo-appstore-new}
PAGE=${PAGE:-https://raw.githubusercontent.com/golang/go/master/README.md}
mkdir -p "$OUT"

# shot SIM NAME JS: launches the app with JS and takes a screenshot after WAIT seconds.
shot() {
    xcrun simctl terminate "$1" $BUNDLE >/dev/null 2>&1
    xcrun simctl launch "$1" $BUNDLE -mardoScript "$3" >/dev/null
    sleep "${WAIT:-17}"
    xcrun simctl io "$1" screenshot "$OUT/$2.png" >/dev/null 2>&1
}
# go SIM NAME JS: opens PAGE, then runs JS six seconds later.
go() {
    local js="const a=document.getElementById('address');a.value='$PAGE';a.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter'}));setTimeout(()=>{const c=document.getElementById('content');if(c)c.scrollTop=0;$3},6000)"
    shot "$1" "$2" "$js"
}
CLOSE="for(const [m,b] of [['settings-menu','menu'],['reading-menu','reading'],['bookmarks-panel','bookmarks-close'],['ask-panel','ask-close']]){const e=document.getElementById(m);if(e&&!e.hidden&&getComputedStyle(e).display!=='none')document.getElementById(b).click()};"
# theme Dark|Light JS: picks the theme in the menu, closes the menu, then runs JS.
theme() { echo "document.getElementById('menu').click();setTimeout(()=>{[...document.querySelectorAll('#settings-menu *')].find(e=>e.children.length<3&&e.textContent.trim()==='$1')?.click();setTimeout(()=>{$CLOSE setTimeout(()=>{$2},600)},800)},800)"; }
# Bookmarks to seed: two folders and five public READMEs.
SEED="const U=[['golang/go/master','Languages'],['denoland/deno/main','Languages'],['nodejs/node/main','Languages'],['microsoft/vscode/main','Tools'],['kubernetes/kubernetes/master','Tools']];const F=['Languages','Tools'];let i=0;const bm=()=>{if(i>=U.length)return;const [p,f]=U[i++];const a=document.getElementById('address');a.value='https://raw.githubusercontent.com/'+p+'/README.md';a.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter'}));setTimeout(()=>{document.getElementById('bookmark-add').click();setTimeout(()=>{const s=document.getElementById('bookmark-folder');const o=[...s.options].find(o=>o.textContent.includes(f));if(o)s.value=o.value;document.getElementById('bookmark-save').click();setTimeout(bm,2500)},1500)},5000)};let k=0;const fo=()=>{if(k>=F.length)return bm();document.getElementById('folder-new').click();setTimeout(()=>{document.getElementById('folder-name').value=F[k++];document.getElementById('folder-form').requestSubmit();setTimeout(fo,1200)},800)};fo()"
EXPAND="const p=document.getElementById('bookmarks-panel');if(p.hidden||getComputedStyle(p).display==='none')document.getElementById('bookmarks-toggle').click();const ex=()=>{const c=[...document.querySelectorAll('#bookmarks-tree .folder .tree-label')].find(b=>b.textContent.includes('▸'));if(c){c.click();setTimeout(ex,1200)}};setTimeout(ex,1000)"

device() { # SIM PREFIX
    local sim=$1 n=$2
    xcrun simctl bootstatus "$sim" -b >/dev/null 2>&1
    xcrun simctl status_bar "$sim" override --time 9:41 --batteryState charged --batteryLevel 100 --cellularBars 4 --wifiBars 3 >/dev/null 2>&1
    xcrun simctl uninstall "$sim" $BUNDLE >/dev/null 2>&1   # a clean start: no tabs, bookmarks or settings left over
    xcrun simctl install "$sim" "$APP" || return 1
    echo "$n: seeding bookmarks"
    WAIT=75 shot "$sim" "$n-seed" "$SEED"
    rm -f "$OUT/$n-seed.png"
    echo "$n: dark set"
    go "$sim" "$n-1-reading-dark" "$(theme Dark "")"
    go "$sim" "$n-2-ai-tools-dark" "$(theme Dark "document.getElementById('reading').click()")"
    go "$sim" "$n-3-ask-dark" "$(theme Dark "document.getElementById('ask-toggle').click()")"
    go "$sim" "$n-4-menu-dark" "$(theme Dark "document.getElementById('menu').click()")"
    echo "$n: light set"
    go "$sim" "$n-5-reading-light" "$(theme Light "")"
    go "$sim" "$n-6-ai-tools-light" "$(theme Light "document.getElementById('reading').click()")"
    WAIT=21 go "$sim" "$n-7-bookmarks" "$EXPAND"
    go "$sim" "$n-8-markdown-source" "$CLOSE setTimeout(()=>document.getElementById('source').click(),400)"
    xcrun simctl status_bar "$sim" clear >/dev/null 2>&1
}

case "${1:-all}" in
    phone) device "$PHONE" iphone ;;
    pad) device "$PAD" ipad ;;
    *) device "$PHONE" iphone; device "$PAD" ipad ;;
esac
echo "done: $OUT"
for f in "$OUT"/*.png; do echo "$(basename "$f") $(sips -g pixelWidth -g pixelHeight "$f" | awk '/pixel/{printf $2"x"}')"; done
