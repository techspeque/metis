#!/bin/sh
# A fake remote launcher for metis auto tests. It runs the fake agent the
# way a launcher on another machine would: in its own clone of origin
# ($FAKE_STATE/node), at the caller's current branch, pushing the agent's
# commits back before it returns. It never touches the caller's checkout.
# Usage: fake-remote.sh <prompt-file> <slug>
url=$(git remote get-url origin) || exit 93
branch=$(git rev-parse --abbrev-ref HEAD)
node="$FAKE_STATE/node"
if [ ! -d "$node" ]; then
	git clone -q "$url" "$node" 2>/dev/null || exit 93
	git -C "$node" config user.email node@t.co
	git -C "$node" config user.name Node
fi
cd "$node" || exit 93
git fetch -q origin "$branch" || exit 93
git checkout -q "$branch" 2>/dev/null || git checkout -q -b "$branch" FETCH_HEAD || exit 93
git merge -q --ff-only FETCH_HEAD || exit 93
sh "$(dirname "$0")/fake-agent.sh" "$@"
code=$?
# A session lost before its push: the commits stay here until a test
# pushes them, late.
[ -f "$FAKE_STATE/hold-push" ] && exit 95
git push -q origin "$branch" || exit 94
exit $code
