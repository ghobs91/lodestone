#!/usr/bin/env bash
#
# Point git at .githooks/ so commits trigger scripts/deploy.sh on every host.
#
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_DIR"

chmod +x .githooks/hook scripts/deploy.sh

# git only runs hooks for names it recognises, so register each one.
# The symlink is relative to .git/hooks/, hence two levels up.
for hook in post-commit; do
  ln -sf ../../.githooks/hook ".git/hooks/$hook"
done

# Symlinks only take effect on the default hooks path. Only touch the config if
# it is unset, so an intentional custom hooksPath is never clobbered.
current_hooks_path="$(git config --get core.hooksPath || true)"
if [ -z "$current_hooks_path" ]; then
  git config core.hooksPath .git/hooks
elif [ "$current_hooks_path" != ".git/hooks" ]; then
  echo "warning: core.hooksPath is '$current_hooks_path'; hooks were linked into .git/hooks" >&2
fi

echo "installed hooks: post-commit -> .githooks/hook"
for hook in post-commit; do
  if [ ! -x ".git/hooks/$hook" ]; then
    echo "error: .git/hooks/$hook is not executable (dangling symlink?)" >&2
    exit 1
  fi
done
echo "deploys run detached; check .deploy/last.log"
echo "skip a single deploy with: LODESTONE_SKIP_DEPLOY=1 git commit ..."
