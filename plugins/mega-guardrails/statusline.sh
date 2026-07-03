#!/bin/bash
# Claude Code statusline: folder | branch | git changes | model + effort | context % | mem/cpu/disk | 5h usage (reset) | weekly usage (reset)
input=$(cat)
j() { echo "$input" | jq -r "$1"; }

G='\033[32m'; Y='\033[33m'; R='\033[31m'; C='\033[36m'; B='\033[34m'; M='\033[35m'; D='\033[2m'; X='\033[0m'
col() { local p=${1%.*}; if [ "${p:-0}" -ge 90 ]; then printf "$R"; elif [ "${p:-0}" -ge 70 ]; then printf "$Y"; else printf "$G"; fi; }

WD=$(j '.workspace.current_dir // .cwd')
DIR=${WD##*/}
CTX=$(j '.context_window.used_percentage // 0'); CTX=${CTX%.*}
BRANCH=$(git -C "$WD" branch --show-current 2>/dev/null)
MODEL=$(j '.model.display_name // empty')
EFFORT=$(j '.effort.level // empty')

GIT_SEG=""
if git -C "$WD" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  read -r CF ADD DEL <<<"$(git -C "$WD" diff HEAD --shortstat 2>/dev/null \
    | awk '{f=a=d=0; for(i=2;i<=NF;i++){if($i~/^file/)f=$(i-1); else if($i~/^insertion/)a=$(i-1); else if($i~/^deletion/)d=$(i-1)} print f,a,d}')"
  CF=${CF:-0}; ADD=${ADD:-0}; DEL=${DEL:-0}
  GIT_SEG="${CF}f ${G}+${ADD}${X}/${R}-${DEL}${X}"
fi

# system: memory used%, cpu (1m load normalized to cores)%, disk used%
MEM=$(awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} END{if(t>0) printf "%d", (t-a)*100/t}' /proc/meminfo 2>/dev/null)
read -r LOAD1 _ </proc/loadavg 2>/dev/null
CPU=$(awk -v l="$LOAD1" -v n="$(nproc 2>/dev/null)" 'BEGIN{if(n>0) printf "%d", l*100/n}')
DISK=$(df -P "$WD" 2>/dev/null | awk 'NR==2{gsub(/%/,"",$5); print $5}')

fmt_reset() { [ -n "$1" ] && date -d @"$1" '+%a %H:%M' 2>/dev/null; }
H5=$(j '.rate_limits.five_hour.used_percentage // empty')
H5R=$(fmt_reset "$(j '.rate_limits.five_hour.resets_at // empty')")
WK=$(j '.rate_limits.seven_day.used_percentage // empty')
WKR=$(fmt_reset "$(j '.rate_limits.seven_day.resets_at // empty')")

OUT="${B}${DIR}${X}"
[ -n "$BRANCH" ] && OUT="$OUT ${C}${BRANCH}${X}"
[ -n "$GIT_SEG" ] && OUT="$OUT $GIT_SEG"
OUT="$OUT │ ${M}${MODEL:-?}${X}${EFFORT:+ ${D}${EFFORT}${X}}"
OUT="$OUT │ $(col "$CTX")ctx ${CTX}%${X}"
SYS=""
[ -n "$MEM" ] && SYS="$SYS $(col "$MEM")mem ${MEM}%${X}"
[ -n "$CPU" ] && SYS="$SYS $(col "$CPU")cpu ${CPU}%${X}"
[ -n "$DISK" ] && SYS="$SYS $(col "$DISK")disk ${DISK}%${X}"
[ -n "$SYS" ] && OUT="$OUT │${SYS}"
[ -n "$H5" ] && OUT="$OUT │ $(col "$H5")5h ${H5%.*}%${X}${H5R:+ ↻$H5R}"
[ -n "$WK" ] && OUT="$OUT │ $(col "$WK")wk ${WK%.*}%${X}${WKR:+ ↻$WKR}"
echo -e "$OUT"
