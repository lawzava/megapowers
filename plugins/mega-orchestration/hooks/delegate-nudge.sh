#!/usr/bin/env bash
# Stop-hook backstop for risky pending changes. Only a current, independent,
# launcher-generated approval receipt suppresses the nudge.
set -u
# REPLACEMENT OBJECTS, off for every git call this gate makes and for the
# review-diff-id it runs. `git replace X Y` makes every object read return Y
# where X was asked for and moves no ref, so `git rev-parse HEAD` still prints X
# while `git diff HEAD` renders against Y. The receipt binds subject.base to the
# HEAD ref OID, so without this a second repository holding X plus a replacement
# to Y reproduces the approved delta and satisfies artifact, base and id although
# its real content was never reviewed. Exported, so the fingerprint subprocess
# inherits it; review-diff-id sets it itself as well and carries the full
# reasoning, including why this beats binding HEAD^{tree}.
export GIT_NO_REPLACE_OBJECTS=1
input="$(cat)"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=stop-context.sh
. "$here/stop-context.sh"

command -v jq >/dev/null 2>&1 || exit 0
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false')" = "true" ] && exit 0
stop_context_is_exempt "$input" && exit 0

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0
# The gate is about changed logic, so it reads code paths only. Prose names these
# categories constantly: a checklist that lists them, or a template telling an
# agent to route such work for review, is not a change to that work. Scanning
# documentation fired the gate on doc-only edits.
#
# This file and its tests are excluded for the same reason, one step further in:
# they must contain the risky keyword list verbatim (the pattern below, the block
# message that names the categories, and fixtures like `billing()` that prove the
# gate fires). Without this, editing the guard always trips the guard, and the
# resulting review request cites its own warning text as the risky change. The
# exclusion is deliberately narrow: sibling hooks stay scanned, so a real logic
# change to deny-destructive.sh is still gated, and it only ever matches inside a
# checkout of this repository.
prose=(':(top,exclude,glob)**/*.md' ':(top,exclude,glob)**/*.mdx'
       ':(top,exclude,glob)**/*.markdown' ':(top,exclude,glob)**/*.rst'
       ':(top,exclude,glob)**/mega-orchestration/hooks/delegate-nudge.sh'
       ':(top,exclude,glob)**/mega-orchestration/hooks/tests/**')
# One diff, three call sites (compute, retry, diagnose), so the command lives in
# one place: a flag or base added to only two of the three is the kind of drift
# that silently narrows what the gate reads. Redirection stays at the call sites,
# because each one wants something different from stderr.
base=HEAD
tracked_diff() { git diff "$base" --binary --no-ext-diff -- ':/' "${prose[@]}"; }
# Hoisted: the tracked enumeration below wants it, and so do the index-flag and
# submodule checks, which run on every stop rather than only on an unreadable
# diff.
root="$(git rev-parse --show-toplevel 2>/dev/null)"

# The status is captured separately from the output, because git prints NOTHING
# when it aborts: one tracked path it cannot hash and the whole diff comes back
# empty with status 128. Read as output alone, that empty result is
# indistinguishable from a clean tree, and the guard would pass a diff it never
# computed. The sandbox creates the condition routinely by bind mounting
# /dev/null over deny-listed paths, which turns a tracked file like .env.example
# into a character device.
diff="$(tracked_diff 2>/dev/null)"
rc=$?
# A repository with no commits has no HEAD to diff against, so `git diff HEAD`
# always fails there for an ordinary reason. That is not an unreadable tree, but
# it is not an empty one either: the index can already hold staged content, which
# `git ls-files --others` does not report, so calling the diff empty here would
# pass staged risky logic unread. Re-base onto the empty tree instead, which
# every repository has, so the whole index stays visible.
if [ "$rc" -ne 0 ] && ! git rev-parse --verify -q HEAD >/dev/null 2>&1; then
  base="$(git hash-object -t tree /dev/null 2>/dev/null)"
  diff="$(tracked_diff 2>/dev/null)"
  rc=$?
fi
skipped=()
if [ "$rc" -ne 0 ]; then
  # A tracked path that exists but is not a regular file is a character device,
  # fifo, or socket, so it cannot carry logic: drop it from the pathspec and
  # recompute. Symlinks stay in (git diffs the link target string, not the file
  # behind it), and so do paths missing from the worktree, because a deletion is a
  # real change and git reports it without hashing anything.
  #
  # Enumerate from the repo root with --full-name: `:(top,...)` resolves against
  # the root, while plain `git ls-files` prints paths relative to the working
  # directory and lists only what sits under it. Run from a subdirectory,
  # cwd-relative names would exclude nothing and every stop would demand a review.
  while IFS= read -r -d '' p; do
    [ -L "$root/$p" ] && continue
    [ -e "$root/$p" ] || continue
    [ -f "$root/$p" ] && continue
    # Matches review-diff-id: a submodule gitlink is a directory in the worktree
    # and fails `[ -f ]`, and its diff is a real change to a pointer that carries
    # arbitrary code, so it must never land in the excluded set.
    [ -d "$root/$p" ] && continue
    # `literal` is mandatory, not tidiness. Without it the name is matched with
    # wildmatch and `*` crosses directory separators, so a fifo called `x*.go`
    # drops every path starting with `x`, `xdir/deep/billing.go` included, from
    # the diff this gate reads: a risky change then leaves no trace and the hook
    # allows in silence. The name is picked by whoever adds the file, so the glob
    # chooses what goes unreviewed. It is also the semantics the code around it
    # already assumes, since `p` is a verbatim `git ls-files` name tested with
    # `-e` and `-f`.
    skipped+=(":(top,exclude,literal)$p")
  done < <(git ls-files -z --full-name -- ':/' 2>/dev/null)
  if [ "${#skipped[@]}" -gt 0 ]; then
    prose+=("${skipped[@]}")
    diff="$(tracked_diff 2>/dev/null)"
    rc=$?
  fi
fi
# Read NUL-delimited, like the tracked enumeration above. With the default
# core.quotePath git QUOTES a name holding a newline, tab, quote, or a non-ASCII
# byte, and that display form names no file on disk: read newline-delimited, the
# `[ -f ]` test below fails, the file is never opened, and risky content inside it
# is never scanned. The name is picked by whoever adds the file, so quoting alone
# chooses what goes unread. -z emits raw names and the problem does not arise.
untracked=()
while IFS= read -r -d '' f; do untracked+=("$f"); done \
  < <(git ls-files -z --others --exclude-standard -- ':/' "${prose[@]}" 2>/dev/null)
if [ "$rc" -ne 0 ]; then
  # Still unreadable, so the gate has no idea what changed. Asking on an
  # uncomputable diff is the only safe direction: reporting clean here is how a
  # risky change ships unreviewed.
  #
  # The reason must name the only escape that exists. This block sits ahead of the
  # receipt check, and review-diff-id cannot produce an id for a tree it cannot
  # read either, so no receipt can ever clear it. Telling the agent to go get a
  # review here would send it after a receipt that does nothing, which is pressure
  # to misrepresent what it did in order to leave a gate it cannot satisfy.
  detail="$(tracked_diff 2>&1 >/dev/null | head -3 | tr '\n' ' ')"
  jq -nc --arg detail "$detail" \
    '{decision:"block", reason:("This gate could not compute the pending diff, so it could not check whether risky auth, billing, payment, or concurrency logic changed. git reported: " + $detail + " Do not treat the tree as clean. Inspect the pending changes yourself. A review receipt cannot clear this block, so either make the path git named readable and stop again, or tell the human plainly that the automated check could not read the diff, which path caused it, and what your own inspection found.")}'
  exit 0
fi

# THE INDEX CAN HIDE A MODIFICATION FROM EVERY READ ABOVE.
#
# `git update-index --assume-unchanged P` tells git to trust the index entry for
# P and stop stat'ing the worktree. An edit to P after that appears in no
# `git diff`, in no `git status`, in no untracked listing, and in no fingerprint:
# this gate reads a clean tree and exits, or keeps honoring a receipt written
# before the edit. That is a tracked-file change shipping with no review at all.
#
# The neighbouring skip-worktree bit is NOT the same thing and is not refused
# with it. Sparse checkout sets skip-worktree on every path outside the cone, so
# a blanket refusal would block every stop in a sparse checkout. `git ls-files -v`
# separates them: a LOWERCASE tag is assume-unchanged, uppercase 'S' is
# skip-worktree, and 's' is both.
#
# Content is the trigger, not the bit. A bit set on a path whose worktree content
# still matches the index hides nothing at this stop, and this runs again at the
# next one, so a later edit is caught then. Refusing on the bit alone would fire
# on every stop in a repository that merely uses the feature, and a gate that
# fires on nothing is a gate that gets disabled.
# ONE index pass serves this check AND the submodule scan below. `-s -v -z`
# prints `<tag> <mode> <sha> <stage>\t<path>\0`, so the flag, the recorded blob
# and the gitlink mode all arrive together and the gate does not read the index
# three times on every stop.
# PREFILTER, because this runs on every stop. A NUL-delimited bash read loop over
# a whole index costs about half a second on twenty thousand paths, and almost
# every repository has neither an index flag nor a gitlink, so the loop is doing
# nothing in the case that matters for latency. `git ls-files` C-QUOTES a name
# holding a newline (it does so regardless of core.quotePath), so a
# newline-delimited listing puts every index entry on exactly one line: grep can
# then decide whether anything here is worth a precise pass, with no false
# negatives, at C speed. This is only ever a gate on doing the work; the precise
# pass below reads the NUL form and remains the only thing that parses a path.
#
# `--full-name`, in both listings. Without it `git ls-files` prints names relative
# to the CURRENT DIRECTORY and lists only what sits under it, so a stop from a
# subdirectory would test the wrong paths and see none of the gitlinks above it.
index_lines="$(mktemp 2>/dev/null)" || index_lines=""
index_list=""
hidden=""
hidden_rc=0
sub_risky=0
if [ -z "$index_lines" ] || ! git ls-files -s -v --full-name -- ':/' > "$index_lines" 2>/dev/null; then
  hidden_rc=1
elif LC_ALL=C grep -qE '^([^H] |H 160000 )' "$index_lines"; then
  # The prefilter says there IS something here, so failing to get scratch for the
  # precise pass is not a reason to skip it in silence: that is the difference
  # between "nothing to check" and "could not check".
  index_list="$(mktemp 2>/dev/null)" || { index_list=""; hidden_rc=1; }
fi
[ -z "$index_lines" ] || rm -f "$index_lines"
if [ -n "$index_list" ] && ! git ls-files -s -v -z --full-name -- ':/' > "$index_list" 2>/dev/null; then
  hidden_rc=1
  rm -f "$index_list"
  index_list=""
fi
if [ -n "$index_list" ]; then
  sparse="$(git config --bool core.sparseCheckout 2>/dev/null || true)"
  while IFS= read -r -d '' entry; do
    tag="${entry%% *}"
    rest="${entry#* }"
    mode="${rest%% *}"
    path="${rest#*$'\t'}"

    # SUBMODULES DIFF AS FLAGS, NOT AS CONTENT. A gitlink renders as a bare
    # `Subproject commit <sha>` line plus a `-dirty` suffix, so at a neutral path
    # like vendor/lib the risky scan below matches NOTHING and this gate exits
    # clean over a pointer bump that pulled in arbitrary code. Untracked content
    # inside a submodule is worse: under the default configuration it reaches the
    # superproject diff nowhere at all, not even as a flag.
    #
    # So any gitlink that is changed or dirty is risky by construction. What the
    # reviewer then reads is the submodule section delegate-run puts in the review
    # package, bound by subject.submodules, which the receipt check below
    # verifies. A clean, unchanged gitlink is not risky and costs nothing here.
    if [ "$mode" = "160000" ] && [ "$sub_risky" -eq 0 ]; then
      # `--ignore-submodules=none` on the COMMAND LINE, not via a `-c` variable.
      # `submodule.<name>.ignore` lives in the checked-in .gitmodules and beats
      # `diff.ignoreSubmodules`, so a repository shipping `ignore = all` hides its
      # own submodule from every diff, pointer bump included. Only the option form
      # overrides it, and what this gate reads must not be the reviewed
      # repository's choice.
      if ! git diff --cached --quiet --ignore-submodules=none -- ":(top,literal)$path" 2>/dev/null ||
         ! git diff --quiet --ignore-submodules=none -- ":(top,literal)$path" 2>/dev/null; then
        sub_risky=1
      elif [ -d "$root/$path" ]; then
        # The pointer matches, so only the submodule's own worktree is left.
        #
        # `--show-toplevel` compared against the gitlink path, NOT `--git-dir`. An
        # UNINITIALIZED submodule is a plain directory inside the superproject's
        # worktree, so --git-dir there walks up and answers with the
        # superproject's own git directory: the status below would then report the
        # superproject's pending changes as the submodule's and gate every stop.
        sub_top="$(git -C "$root/$path" rev-parse --show-toplevel 2>/dev/null)" || sub_top=""
        sub_top="$(cd "$sub_top" 2>/dev/null && pwd -P)" || sub_top=""
        sub_phys="$(cd "$root/$path" 2>/dev/null && pwd -P)" || sub_phys=""
        if [ -n "$sub_top" ] && [ "$sub_top" = "$sub_phys" ]; then
          # `--untracked-files=normal` and `--ignore-submodules=none` explicitly: a
          # `status.showUntrackedFiles=no` or an ignore setting inside the
          # submodule would otherwise decide what this gate is allowed to notice.
          [ -z "$(git -C "$root/$path" status --porcelain --untracked-files=normal \
                    --ignore-submodules=none 2>/dev/null)" ] || sub_risky=1
        elif [ -n "$(ls -A "$root/$path" 2>/dev/null)" ]; then
          # Present, not a repository, and NOT EMPTY. git refuses to look inside a
          # gitlink path at all, so `ls-files --others` never descends and status
          # says nothing: whatever is in here is invisible to the diff above, to
          # the untracked scan below, and to the fingerprint. Uninitialized and
          # empty is the ordinary case and stays silent; uninitialized and
          # occupied is content nobody can see.
          sub_risky=1
        fi
      fi
    fi

    # THE INDEX CAN HIDE A MODIFICATION FROM EVERY READ ABOVE.
    #
    # `git update-index --assume-unchanged P` tells git to trust the index entry
    # for P and stop stat'ing the worktree. An edit to P after that appears in no
    # `git diff`, in no `git status`, in no untracked listing, and in no
    # fingerprint: this gate reads a clean tree and exits, or keeps honoring a
    # receipt written before the edit. That is a tracked-file change shipping with
    # no review at all.
    #
    # The neighbouring skip-worktree bit is NOT the same thing and is not refused
    # with it. Sparse checkout sets skip-worktree on every path outside the cone,
    # so a blanket refusal would block every stop in a sparse checkout. The tag
    # separates them: a LOWERCASE tag is assume-unchanged, uppercase 'S' is
    # skip-worktree, and 's' is both.
    #
    # Content is the trigger, not the bit. A bit set on a path whose worktree
    # content still matches the index hides nothing at this stop, and this runs
    # again at the next one, so a later edit is caught then. Refusing on the bit
    # alone would fire on every stop in a repository that merely uses the feature,
    # and a gate that fires on nothing is a gate that gets disabled.
    case "$tag" in
      [a-z]) assume=1 ;;
      S) assume=0 ;;
      *) continue ;;
    esac
    if [ ! -e "$root/$path" ] && [ ! -L "$root/$path" ]; then
      # Absent is how sparse checkout removes a path, so it is only acceptable
      # when the skip-worktree bit is involved and the checkout is sparse. Under
      # assume-unchanged alone it is a DELETION no diff reports.
      if [ "$assume" -eq 0 ] || [ "$tag" = "s" ]; then
        [ "$sparse" = "true" ] || hidden="$hidden $path"
      else
        hidden="$hidden $path"
      fi
      continue
    fi
    if [ -L "$root/$path" ] || [ ! -f "$root/$path" ] || [ ! -r "$root/$path" ]; then
      # Not comparable against the index entry, so whether it hides a change is
      # unknown, and unknown must not read as clean. A gitlink lands here too and
      # is covered by the submodule scan above rather than by a content compare.
      [ "$mode" = "160000" ] || hidden="$hidden $path"
      continue
    fi
    # `<mode> <sha> <stage>\t<path>`: drop the mode, keep the object name.
    idx="${rest#* }"
    idx="${idx%% *}"
    # `--path`, so a clean filter configured for this name runs the way it ran
    # when the index entry was written.
    work="$(git hash-object --path "$path" -- "$root/$path" 2>/dev/null)" || work=""
    [ -n "$idx" ] && [ -n "$work" ] && [ "$idx" = "$work" ] || hidden="$hidden $path"
  done < "$index_list"
  rm -f "$index_list"
fi
if [ -n "$hidden" ] || [ "$hidden_rc" -ne 0 ]; then
  # No receipt clears this, and the reason says so: review-diff-id refuses to
  # fingerprint such a tree for the same reason, so sending the agent after a
  # review here would send it after a receipt that cannot be produced.
  detail="the index hides worktree changes at$hidden"
  [ -n "$hidden" ] || detail="this gate could not read the index flags, so it cannot tell whether the index is hiding a worktree change"
  jq -nc --arg detail "$detail" \
    '{decision:"block", reason:("Pending changes cannot be checked for risky auth, billing, payment, or concurrency logic: " + $detail + ". A path under assume-unchanged is omitted from every diff, from the risky-content scan, and from the review fingerprint, so this gate would report a clean tree over an edit nobody read. Clear the bit with '"'"'git update-index --no-assume-unchanged <path>'"'"' (or --no-skip-worktree) and stop again, then inspect what it exposes. A review receipt cannot clear this block.")}'
  exit 0
fi

[ -n "$diff" ] || [ "${#untracked[@]}" -gt 0 ] || [ "$sub_risky" -eq 1 ] || exit 0

risky='authn|authz|authenticat|authoriz|oauth|jwt|saml|passwd|password|billing|payment|invoice|subscription|stripe|webhook|mutex|goroutine|semaphore|deadlock|concurren'
hit="$sub_risky"
printf '%s' "$diff" | grep -qiE "$risky" && hit=1
if [ "${#untracked[@]}" -gt 0 ]; then
  printf '%s\n' "${untracked[@]}" | grep -qiE "$risky" && hit=1
  # EVERY regular untracked file is opened. Stopping at the first fifty paths let
  # path fifty-one carry risky content past the scan with nothing said, and a
  # silent bound on a security scan is a hole wherever it is drawn. The batching
  # is only about the execve argument limit, not about how much gets read, and the
  # loop stops early once something has already been found.
  #
  # `[ -f ]` is the filter that matters: it keeps grep off a fifo, which would
  # block forever and hang the Stop hook. Symlinks to regular files still read,
  # which is what git would diff anyway.
  scan_files=()
  for f in "${untracked[@]}"; do
    [ -f "$f" ] || continue
    # A regular file the scan cannot open is content the gate has not read, and
    # that is the same silence as skipping it: grep writes an error nobody sees
    # and reports no match, so unknown content reads as benign. Treat it as risky
    # so the ordinary remedy applies rather than inventing a block nothing clears.
    if [ -r "$f" ]; then scan_files+=("$f"); else hit=1; fi
  done
  i=0
  while [ "$hit" -eq 0 ] && [ "$i" -lt "${#scan_files[@]}" ]; do
    grep -qiE "$risky" -- "${scan_files[@]:i:200}" 2>/dev/null && hit=1
    i=$((i + 200))
  done
fi
[ "$hit" -eq 1 ] || exit 0

diff_id_tool="$here/../skills/multi-agent-delegation/scripts/review-diff-id"
receipt="$(git rev-parse --git-path megapowers-review-receipt.json 2>/dev/null)"
receipt_schema=""
if [ -x "$diff_id_tool" ] && [ -f "$receipt" ]; then
  receipt_schema="$(jq -r '.schema // ""' "$receipt" 2>/dev/null)"
  current_id="$("$diff_id_tool" . 2>/dev/null)"
  # The SUPPLEMENTAL submodule fingerprint, checked beside the id rather than
  # folded into it. subject.id must keep the value the shipped algorithm makes,
  # because receipts are compared against it, so submodule content is bound in a
  # second field. Empty output with status 0 means the tree holds no gitlink, and
  # that empty matches a receipt with no `submodules` key: an ordinary repository
  # keeps clearing a receipt written before this field existed. A non-zero status
  # means the snapshot could not be read at all, which must not read as "no
  # submodules", so it fails the check below and the gate blocks.
  sub_ok=1
  current_sub="$("$diff_id_tool" --submodules . 2>/dev/null)" || sub_ok=0
  # WHAT THE ID DOES AND DOES NOT SAY. The fingerprint covers the pending DELTA:
  # the staged diff, the unstaged diff, and the untracked files. It says nothing
  # about what that delta is applied to. A delta is therefore a shape, not a tree,
  # and unrelated repositories that happen to carry the same pending hunks
  # fingerprint identically. Binding the worktree PATH does not fix that, because a
  # path is a location: move one checkout away, put a different repository at the
  # same path, hand it the first one's receipt, and both the id and the path match
  # while nothing about the code does.
  #
  # So bind the BASE. Base commit plus delta determines the complete tracked
  # content, which is what "this change, on this tree" actually means. A different
  # repository at the same path has a different HEAD and cannot match; the same
  # repository advanced by a commit has a different HEAD and cannot match either,
  # which is correct, because committing changes what is pending.
  current_base="$(git rev-parse --verify -q HEAD 2>/dev/null)"
  # No commits: the empty tree is the base git itself diffs the index against, and
  # it is what delegate-run recorded in that case. Matching there is sound because
  # with no commits the delta alone already determines the whole tree.
  [ -n "$current_base" ] || current_base="$(git hash-object -t tree /dev/null 2>/dev/null)"
  # The worktree path stays in as a second, narrower condition. It binds the wrong
  # thing on its own, but it only ever rejects, and a receipt that has to match
  # both is never more permissive than one that matches the base alone.
  #
  # Both sides resolve the same way delegate-run built the field (an absolute
  # `git rev-parse --show-toplevel`), then physically, so a symlinked or otherwise
  # differently spelled path to the same worktree is not a false mismatch.
  current_root="$(git rev-parse --show-toplevel 2>/dev/null)"
  current_root="$(cd "$current_root" 2>/dev/null && pwd -P)"
  receipt_root="$(jq -r '.subject.artifact // ""' "$receipt" 2>/dev/null)"
  case "$receipt_root" in
    # A relative artifact names whatever directory it is read from, so it would
    # clear this check in every repository at once, which is the portability the
    # check exists to remove. The launcher never writes one.
    /*) receipt_root="$(cd "$receipt_root" 2>/dev/null && pwd -P)" ;;
    *) receipt_root="" ;;
  esac
  # v2 ONLY. A v1 receipt records no base, so honoring one would keep exactly the
  # hole this check closes, and there is no way to infer the base it covered after
  # the fact. Rejecting costs one re-run of the launcher; accepting costs a tree
  # nobody reviewed. See the block reason below, which names the reason rather than
  # letting a receipt stop working in silence.
  if [ -n "$current_id" ] && [ -n "$current_root" ] && [ -n "$current_base" ] && \
     [ "$sub_ok" -eq 1 ] && \
     [ "$receipt_root" = "$current_root" ] && jq -e --arg id "$current_id" --arg base "$current_base" \
     --arg sub "$current_sub" '
    . as $receipt |
    .schema == "megapowers.review-receipt.v2" and
    (.role == "verify" or .role == "code_review" or .role == "visual_verify") and
    .subject.kind == "worktree-diff" and .subject.id == $id and
    .subject.base == $base and
    (.subject.submodules // "") == $sub and
    .independent == true and .result.verdict == "approve" and
    (.author_vendors | type == "array" and length > 0) and
    (.reviewer.vendor | type == "string" and length > 0) and
    (all(.author_vendors[]; . != $receipt.reviewer.vendor))
  ' "$receipt" >/dev/null 2>&1; then
    exit 0
  fi
fi

# Named explicitly, because a receipt that quietly stops working reads as a bug in
# the gate and invites working around it.
legacy=""
if [ "$receipt_schema" = "megapowers.review-receipt.v1" ]; then
  legacy="A megapowers.review-receipt.v1 receipt is present and is not honored: v1 records no base commit, so it cannot say which tree state its reviewer read, and the same receipt clears any checkout carrying the same pending delta. Re-run the review to earn a v2 receipt. "
fi

launcher="$here/../skills/multi-agent-delegation/scripts/delegate-run"
resolver="$here/../skills/multi-agent-delegation/scripts/delegate-resolve"

# Cross-vendor review needs at least two reachable vendors. With one (or none),
# no --author-vendor choice can route away from the author, so instructing the
# agent to run the launcher would prescribe a command that cannot succeed: it
# would exit 3 no matter what. The risk is real either way, so the gate still
# fires; only the remedy changes to one the agent can actually carry out.
# Probe the verify ROLE, not the machine. A globally installed vendor that the
# verify chain does not route to, or that fails its capability/tier/effort/floor
# requirements, cannot serve the review; counting it would claim an independent
# pass is possible when resolution would exit 3.
#
# Default 2 (the stricter path) so a missing or failing resolver never downgrades
# the remedy. Count only when the resolver itself succeeds, and count with awk
# rather than `grep -c .`: grep exits 1 on zero matches, which would discard a
# legitimate zero and leave the default in place, sending a machine with NO
# reachable vendor down the launcher path it cannot follow.
reachable=2
if [ -x "$resolver" ]; then
  if vendor_list="$("$resolver" verify --vendors 2>/dev/null)"; then
    reachable="$(printf '%s\n' "$vendor_list" | awk 'NF{n++} END{print n+0}')"
  fi
fi

if [ "$reachable" -lt 2 ]; then
  jq -nc --arg legacy "$legacy" \
    '{decision:"block", reason:($legacy + "Risky auth, billing, payment, or concurrency logic changed, and no independent reviewer is reachable: fewer than two delegate vendors have an installed CLI, so a different-vendor review cannot be resolved on this machine. Do not silently ship it. Summarize the risky change and its blast radius for the human and get an explicit go-ahead, or install a second vendor CLI and re-run the independent pass. Say plainly that the automated cross-vendor check did not run.")}'
  exit 0
fi

jq -nc --arg launcher "$launcher" --arg legacy "$legacy" \
  '{decision:"block", reason:($legacy + "Risky auth, billing, payment, or concurrency logic changed without a current independent approval receipt. Run " + $launcher + " --role verify --author-vendor <artifact-author-vendor> --artifact worktree --claim <claim>. The launcher resolves a different-vendor reviewer and binds its verdict to the pending tree git reports, plus the base commit it applies to. Ignored paths and content git does not surface in a diff are outside that binding. Unrelated delegate calls and stale receipts do not satisfy this gate.")}'
exit 0
