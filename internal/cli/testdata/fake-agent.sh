#!/bin/sh
# A fake agent for metis auto tests. It plays the session the next line of
# $FAKE_SCRIPT names, running the real metis binary on $PATH, and counts
# its sessions in $FAKE_STATE. Usage: fake-agent.sh <prompt-file> <slug>
prompt_file="$1"
slug="$2"
slice=$(sed -n 's/^metis handoff · template [^ ]* · slice \([^ ]*\) · .*/\1/p' "$prompt_file")
n=$(( $(cat "$FAKE_STATE/count" 2>/dev/null || echo 0) + 1 ))
echo "$n" > "$FAKE_STATE/count"
action=$(sed -n "${n}p" "$FAKE_SCRIPT")
arg="${action#*:}"
echo "fake $slug: session $n on $slice: $action"

case "$action" in
code)
	if [ ! -f ".metis/briefs/$slice.md" ]; then
		mkdir -p .metis/briefs
		printf '# %s\n\n## Declared file scope\n\n- **owned_paths:** src/%s.txt\n' "$slice" "$slice" > ".metis/briefs/$slice.md"
		metis commit --brief --slice "$slice" || exit 1
	fi
	mkdir -p src
	echo "change $n" >> "src/$slice.txt"
	git add "src/$slice.txt"
	metis commit -m "work $n" --slice "$slice" || exit 1
	metis verify --post --slice "$slice" || exit 1
	metis commit --flip coded --slice "$slice" || exit 1
	;;
pass)
	metis commit --flip reviewed --agent "$slug" --slice "$slice" || exit 1
	;;
block:*)
	metis block "$slice" --severity P2 --category tests --finding "$arg" || exit 1
	;;
p1:*)
	metis block "$slice" --severity P1 --category security --finding "$arg" || exit 1
	;;
human:*)
	metis wait "$slice" --reason "$arg" --human || exit 1
	;;
fail)
	exit 1
	;;
noop)
	;;
*)
	echo "fake agent: unknown action '$action'"
	exit 2
	;;
esac
