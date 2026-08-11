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
# One definition of pending, four call sites (compute, retry, diagnose, reduced
# rescan). A flag or a leg added to only some of them silently narrows what the
# gate reads. Redirection stays at the call sites; each wants different stderr.
#
# THE SUBJECT IS THE INDEX AND THE WORKTREE, NOT THE WORKTREE ALONE. `git diff
# HEAD` says nothing about what is staged, so staging `billing()` and restoring
# the worktree copy to its committed bytes made the two cancel: empty diff, clean
# exit, risky content one plain `git commit` from shipping. Two hops instead,
# base to index and index to worktree, because their UNION is what a commit now
# would carry. It is also the decomposition review-diff-id fingerprints and
# delegate-run renders (staged, unstaged, untracked), so the scan, the subject
# id, and the bytes the reviewer reads describe one tree.
#
# THE PATHSPEC CARRIES ONLY PATHS GIT CANNOT HASH, and only on the WORKTREE hop:
# `--cached` never stats the worktree, and excluding a path there would drop its
# INDEX content, ordinary reviewable text, from the scan.
#
# What the scan may LOOK at is decided against the rules file further down, never
# here. A path the scan ignores must still reach this diff, because the review
# package is built from the same pending tree; narrowing here would hide the file
# from the reviewer too. `dropspec` carries that narrowing for the reduced rescan
# alone and is empty everywhere else.
base=HEAD
skipspec=()
dropspec=()
tracked_diff() {
  local staged=0 unstaged=0
  # Both statuses, and both legs always run: a `return` on the first failure would
  # print a partial diff, and a partial diff read as a whole one is the shape of
  # every bug this gate has had.
  git diff --cached "$base" --binary --no-ext-diff -- ':/' \
    ${dropspec[@]+"${dropspec[@]}"} || staged=$?
  git diff --binary --no-ext-diff -- ':/' \
    ${skipspec[@]+"${skipspec[@]}"} ${dropspec[@]+"${dropspec[@]}"} || unstaged=$?
  [ "$staged" -eq 0 ] && [ "$unstaged" -eq 0 ]
}
# Hoisted: the tracked enumeration below wants it, and so do the index-flag and
# submodule checks, which run on every stop rather than only on an unreadable
# diff. The rules file's project layer resolves against it too, so a stop from a
# subdirectory reads the same repository's opt-out as a stop from the root.
root="$(git rev-parse --show-toplevel 2>/dev/null)"

# --- WHAT THIS GATE MAY DO, AND WHAT IT MAY READ -----------------------------
# Both answers live in plugins/mega-orchestration/enforcement.toml, layered like
# models.toml: project .megapowers/enforcement.toml, then user
# ${XDG_CONFIG_HOME:-~/.config}/megapowers/enforcement.toml, then the shipped
# copy, first layer that DEFINES a key winning for that key. Layer precedence is
# the caller's business per lib-toml.sh's header, so the walk lives here.
#
# Why it is configurable at all, and what the 2026-08-05 audit measured:
# hooks/risky-logic-gate.md.
lib_toml="$here/../skills/multi-agent-delegation/scripts/lib-toml.sh"
rules_layers=()
# THE PROJECT LAYER IS READ FROM THE BASE COMMIT, NEVER FROM THE WORKTREE.
#
# Policy taken from the pending tree is policy written by the change under
# review: one commit adding `state = "off"` beside new billing logic silences the
# gate judging it, and every read here happens before the diff is computed, so
# nothing else would notice. Committed content is content a previous stop already
# gated, so `git show HEAD:` resolves this layer. No commits means no trustworthy
# project layer, and the shipped defaults apply.
#
# The user layer stays worktree-readable: it sits outside the repository, reaches
# no diff, and is the machine owner speaking rather than the change speaking.
project_layer=".megapowers/enforcement.toml"
project_rules=""
project_note=""
# Set below only when the lib is readable, so they are declared here: the notice
# builder past the state check reads them unconditionally and `set -u` would
# abort the whole hook on a machine missing lib-toml.sh.
project_changed=0
project_kind=""
# The scan's own findings, declared here for the same reason: policy_notice_exit is
# defined before any scanning happens and reads all three, so an exit through it on
# a clean tree would abort the hook under `set -u`.
hit=0
binary_note=""
unscannable_risky=""
# The path and keyword that made this gate fire, as "<where>\t<keyword>". Empty
# until something matches. Carried into the block reason so a fire can be
# diagnosed by the agent that receives it: a gate that says only "risky logic
# changed" is untunable, because nobody can tell a true fire from a false one
# without re-deriving the scan by hand.
hit_evidence=""
# EVERY SCRATCH FILE THIS HOOK MAKES IS DECLARED HERE AND REMOVED FROM ONE TRAP.
# Five temps: the committed policy copy (lib-toml.sh reads files, not strings),
# two staged blobs, two index enumerations. Freeing them inline only leaks
# whenever the hook leaves early, which the opt-out exit and any interrupted stop
# both do. The inline removals stay so a long stop does not hold finished
# scratch; the trap covers every other way out.
#
# A NAME IS FREE THE MOMENT THE FILE IS GONE, so an inline removal nulls its
# variable too: mktemp elsewhere can take that name back, and a trap still
# holding it would delete a file this hook does not own.
#
# EXIT alone, no signal traps beside it: bash runs EXIT on a fatal signal before
# dying, so an interrupted stop is covered and a TERM handler would only add a
# second way to leave. SIGKILL cannot be caught and nothing here claims to
# survive one.
pending_staged=""
staged_blob=""
index_lines=""
index_list=""
cleanup_temps() {
  rm -f "$project_rules" "$pending_staged" "$staged_blob" "$index_lines" "$index_list" 2>/dev/null
}
trap cleanup_temps EXIT
if [ -r "$lib_toml" ]; then
  # shellcheck source=../skills/multi-agent-delegation/scripts/lib-toml.sh
  . "$lib_toml"
  if [ -n "$root" ] && git rev-parse --verify -q HEAD >/dev/null 2>&1; then
    project_rules="$(mktemp 2>/dev/null)" || project_rules=""
    # `HEAD:<path>` resolves against the repository ROOT, so a stop from a
    # subdirectory reads the same layer as a stop from the top.
    if [ -n "$project_rules" ] && git show "HEAD:$project_layer" > "$project_rules" 2>/dev/null; then
      rules_layers+=("$project_rules")
    else
      rm -f "$project_rules"
      project_rules=""
    fi
  fi
  # A PENDING EDIT TO THE PROJECT LAYER IS IGNORED, AND THE BLOCK SAYS SO.
  # Running the committed policy in silence would hide a policy change from the
  # one human who has to see it, and the whole file is compared rather than each
  # key judged for whether it loosens: ranking a narrowing against its committed
  # value needs an order this file does not have, and the note only ever appears
  # inside a block that already fired.
  if [ -n "$root" ]; then
    project_changed=0
    # Add, edit and delete read differently to the human who has to judge the
    # edit, and the gate already knows which one it is looking at, so it says so
    # rather than making the reader diff the file to find out.
    project_kind=""
    if [ -n "$project_rules" ]; then
      # BOTH SIDES, because `git diff HEAD` compares HEAD against the WORKTREE and
      # says nothing about the index. Staging `state = "off"` and then restoring
      # the worktree copy to its committed bytes left this clean, so the policy
      # edit sat in the index ready to commit with nothing said. The staged value
      # was never HONORED, since the layer is read from `HEAD:` either way, so this
      # was a visibility gap rather than a bypass, and visibility is the whole job
      # of the notice. A worktree copy git cannot read fails the command and reads
      # as changed, which is the direction that tells the human more rather than
      # less.
      git diff --quiet HEAD -- ":(top,literal)$project_layer" 2>/dev/null || project_changed=1
      git diff --cached --quiet HEAD -- ":(top,literal)$project_layer" 2>/dev/null || project_changed=1
      project_kind=edit
      [ -e "$root/$project_layer" ] || project_kind=deletion
    elif [ -e "$root/$project_layer" ] ||
         [ -n "$(git ls-files --cached -- ":(top,literal)$project_layer" 2>/dev/null)" ]; then
      # The index again: a layer staged and then removed from the worktree is one
      # commit away from governing every stop, and `-e` alone saw nothing.
      # `ls-files --cached` and not `git diff --cached HEAD`, because this branch is
      # also the one a repository with no commits takes, where naming HEAD is a
      # fatal error rather than a clean tree.
      project_changed=1
      project_kind=addition
    fi
    if [ "$project_changed" -eq 1 ]; then
      project_note="A pending change to $project_layer was ignored by this gate: a project policy layer counts only as it stands in the base commit, because the change under review must not be able to loosen the gate reviewing it."
      [ -n "$project_rules" ] || project_note="$project_note No committed copy exists, so the shipped defaults applied."
      project_note="$project_note Read that policy edit on its own before treating this block as settled. "
    fi
  fi
  user_rules="${XDG_CONFIG_HOME:-$HOME/.config}/megapowers/enforcement.toml"
  [ -r "$user_rules" ] && rules_layers+=("$user_rules")
  [ -r "$here/../enforcement.toml" ] && rules_layers+=("$here/../enforcement.toml")
fi
# `toml_key_exists_in` before reading, so a layer that sets a key to the empty
# string still wins the key. Testing the VALUE instead would fall through to the
# shipped default and quietly ignore the override.
rule_scalar() {
  local f
  for f in ${rules_layers[@]+"${rules_layers[@]}"}; do
    toml_key_exists_in "$f" "$1" "$2" || continue
    toml_scalar_in "$f" "$1" "$2"
    return 0
  done
  return 1
}
rule_array() {
  local f
  for f in ${rules_layers[@]+"${rules_layers[@]}"}; do
    toml_key_exists_in "$f" "$1" "$2" || continue
    toml_array_in "$f" "$1" "$2"
    return 0
  done
  return 1
}
rule_flag() { [ "$(rule_scalar "$1" "$2")" = "true" ]; }

# `state`, read before any work happens because a project layer alone has to be
# able to turn this off: that is the supported opt-out, and it only counts if it
# costs nothing to exercise. It costs one commit, since the layer read above is
# the committed one, and that is the whole price of not letting a change exempt
# itself. Anything other than "enforced" is advisory, and an
# advisory rule says nothing from a hook; it is stated in the skill the model
# already reads. An ABSENT or unreadable value stays enforced, because a gate
# that disables itself when its own rules go missing is a gate anyone disables by
# deleting a file.
state="$(rule_scalar rules.risky-logic-review state)" || state=""
[ -n "$state" ] || state="enforced"

# A PENDING POLICY EDIT IS NEVER SILENT, whether or not anything else fired.
# The committed-layer rule had a two-step bypass: a pending file carrying only
# `state = "off"` holds no keyword, so it tripped nothing, and once committed it
# exempted the whole repository with the gate never emitting a word. Tightening
# was as quiet, because the state check leaves before any block can carry
# project_note. The design trusts committed content because a human reviewed it,
# so the moment of change has to reach that human.
#
# A NOTICE, NOT A BLOCK: the pending layer buys nothing while the committed one
# governs, so there is nothing to stop, and blocking would re-fire every stop
# until the edit is committed, which is how a gate gets routed around. What was
# missing was visibility, not force. A stop that blocks for a real finding still
# names the edit in its reason.
project_notice=""
if [ "$project_changed" -eq 1 ]; then
  # The pending file's own state, so the notice says which DIRECTION the edit
  # goes. Naming the file alone would leave the reader to diff it, and a
  # loosening and a tightening are not equally urgent to look at.
  #
  # WHICH COPY THAT DIRECTION IS READ FROM. The worktree copy is the answer almost
  # always, because that is the file the author is editing. The exception is the
  # masked case this gate now detects: the worktree copy reads exactly as the
  # committed one while the INDEX holds something else, one plain `git commit` from
  # governing. Reading the worktree there reports the committed state straight back
  # and the notice denies the very edit it just announced.
  pending_state=""
  pending_file=""
  pending_staged=""
  if git diff --quiet HEAD -- ":(top,literal)$project_layer" 2>/dev/null &&
     ! git diff --cached --quiet HEAD -- ":(top,literal)$project_layer" 2>/dev/null; then
    pending_staged="$(mktemp 2>/dev/null)" || pending_staged=""
    if [ -n "$pending_staged" ] && git show ":$project_layer" > "$pending_staged" 2>/dev/null; then
      pending_file="$pending_staged"
    fi
  fi
  [ -n "$pending_file" ] || { [ -r "$root/$project_layer" ] && pending_file="$root/$project_layer"; }
  [ -n "$pending_file" ] &&
    pending_state="$(toml_scalar_in "$pending_file" rules.risky-logic-review state 2>/dev/null)"
  [ -z "$pending_staged" ] || rm -f "$pending_staged"
  pending_staged=""
  project_notice="Policy notice: a pending $project_kind of $project_layer is not honored by this gate, which reads that layer as it stands in the base commit."
  [ -n "$project_rules" ] || project_notice="$project_notice No committed copy exists, so the shipped defaults applied."
  project_notice="$project_notice This stop ran with state = $state."
  case "$project_kind" in
    deletion) project_notice="$project_notice Removing the layer would return this repository to the shipped defaults." ;;
    *) if [ -n "$pending_state" ]; then
         project_notice="$project_notice The pending file would set state = $pending_state."
       else
         project_notice="$project_notice The pending file sets no state of its own."
       fi ;;
  esac
  project_notice="$project_notice Read that edit before it is committed, because from the commit onward it governs every stop and nothing else announces it."
fi
# Every exit that would otherwise say nothing goes through here. A silent exit is
# the whole defect: the policy file changing is worth one statement on its own, and
# so is content this gate could not read. See the design note above the scan for why
# unscannable content is announced here rather than blocked.
#
# Only while nothing else fired. With `hit` set the same sentences ride in the block
# preamble instead, and the one exit that passes with `hit` set is the one a review
# receipt cleared, where a reviewer has already read that content in the package.
policy_notice_exit() {
  local msg="$project_notice"
  if [ -n "$binary_note" ] && [ "$hit" -eq 0 ]; then
    [ -z "$msg" ] || msg="$msg "
    msg="${msg}Unscanned content notice: ${binary_note}Nothing else in the pending tree matched the risky keyword list and no unscannable path names a risky category, so this gate did not block. Unscanned is not the same as safe: if that content carries logic, read it yourself or send it for review."
  fi
  [ -z "$msg" ] || jq -nc --arg m "$msg" '{systemMessage:$m}'
  exit 0
}

[ "$state" = "enforced" ] || policy_notice_exit

keywords=()
while IFS= read -r kw; do
  [ -n "$kw" ] && keywords+=("$kw")
done < <(rule_array rules.risky-logic-review.scope keywords)
rules_note=""
if [ "${#keywords[@]}" -eq 0 ]; then
  # FAIL CLOSED. An empty list matches nothing, and a scan that matches nothing
  # calls every tree benign in silence, which is worse than the false positives
  # this file exists to remove. Keep the shipped list and drop every narrowing
  # with it: the narrowings come from the same unreadable file and cannot be
  # trusted more than the list could. The block then says so, so a broken rules
  # file reads as a broken rules file rather than as a suspicious diff.
  keywords=(authn authz authenticat authoriz oauth jwt saml passwd password
            billing payment invoice subscription stripe webhook mutex goroutine
            semaphore deadlock concurren)
  rules_note="The enforcement rules at plugins/mega-orchestration/enforcement.toml could not be read or define no keywords, so this gate fell back to its built-in list and scanned everything, comments and prose included. Restore that file, or a .megapowers/enforcement.toml layer, before reading this block as a finding about the code. "
  added_only=0
  skip_comments=0
  exclude_globs=()
  self_exclude=()
else
  # Absent narrowings default to OFF, the widest scan, for the same reason the
  # state defaults to enforced: a missing key must not silently buy leniency.
  added_only=0
  skip_comments=0
  rule_flag rules.risky-logic-review.scope added_lines_only && added_only=1
  rule_flag rules.risky-logic-review.scope skip_comments && skip_comments=1
  exclude_globs=()
  while IFS= read -r g; do
    [ -n "$g" ] && exclude_globs+=("$g")
  done < <(rule_array rules.risky-logic-review.scope exclude_globs)
  self_exclude=()
  while IFS= read -r s; do
    [ -n "$s" ] && self_exclude+=("$s")
  done < <(rule_array rules.risky-logic-review.scope self_exclude)
fi
# Keywords are LITERAL substrings matched against a lowercased line, so a list
# entry carrying a regex metacharacter (`c++`) matches itself rather than
# whatever the engine makes of it. Whoever edits the rules file is writing words,
# not patterns.
risky="$(printf '%s\n' "${keywords[@]}" |
  awk '{gsub(/[][\\^$.|?*+(){}]/,"\\\\&"); printf "%s%s", (NR>1?"|":""), tolower($0)} END{print ""}')"

# THE SAME WORDS, MATCHED AGAINST A PATH, for the one caller that has nothing else
# to read: a file whose bytes this gate could not scan. A path is not evidence of
# what a file does, but it is the only evidence left there, and it is the difference
# between a changed favicon.png and a changed payment_processor.bin.
#
# Builtins only, no pipeline, for the reason nul_scan gives: this runs once per
# unscannable changed path and the untracked-cap budget exists to stop per-file
# processes creeping back in. Literal lowercase substrings, matching how the content
# scan escapes its own list, so a keyword carrying a glob character matches itself.
path_is_risky() {
  local lower="${1,,}" k
  for k in ${keywords[@]+"${keywords[@]}"}; do
    case "$lower" in *"${k,,}"*) return 0 ;; esac
  done
  return 1
}

# exclude_globs decides what the KEYWORD SCAN reads and nothing else. `*` crosses
# `/` in a bash case pattern, so `docs/*` already covers `docs/**`.
#
# AN EXTENSION IS A CLAIM, NOT A PROOF: an executable shell payload named
# billing.md made `git mv` the whole bypass. So two cheap properties about the
# bytes are checked, and either one contradicting the exclusion means the file is
# scanned like any other source: the mode bit (the kernel may run it) and a `#!`
# first line (the only in-band claim a text file makes about being a program).
# `read -n 2`, not a full line, or a single-line multi-megabyte document is
# slurped whole on every stop.
#
# An UNREADABLE excluded file is scanned too: not read is not prose, and the name
# is chosen by whoever adds the file. A deletion keeps the exclusion, because
# removed content executes nowhere. A worktree copy that is a fifo, a device, or
# a directory cannot answer, so the claim goes to the STAGED bytes, which are
# what a plain `git commit` ships.
#
# THE MODE BIT IS A CLAIM THE FILESYSTEM MAKES, AND SOME MAKE IT FOR EVERYTHING.
# FAT, many network mounts, and a permissive umask report every file executable;
# read there, the bit revokes every prose exclusion at once and the gate fires on
# documentation. git writes HEAD without the execute bit, so a filesystem
# reporting HEAD executable is reporting noise. Probed once, and only when some
# excluded file claims the bit.
#
# Where the bit is noise the INDEX MODE survives: 100755 is what a commit ships
# and the filesystem cannot fake it. An untracked path has no index entry, so
# there the shebang is the only claim left. A real narrowing, stated rather than
# hidden: reading the bit anyway costs every excluded document in the tree.
exec_bit_ok=-1
exec_bit_reliable() {
  local probe
  if [ "$exec_bit_ok" -lt 0 ]; then
    exec_bit_ok=1
    probe="$(git rev-parse --git-path HEAD 2>/dev/null)" || probe=""
    [ -n "$probe" ] && [ -f "$probe" ] && [ -x "$probe" ] && exec_bit_ok=0
  fi
  [ "$exec_bit_ok" -eq 1 ]
}
exclude_claim_holds() {
  local f="$root/$1" head2="" mode=""
  # NO REGULAR WORKTREE COPY, SO THE WORKTREE ANSWERS NOTHING AND THE INDEX DOES.
  #
  # A narrowing has to PROVE a property. A fifo, a character device, a socket or a
  # directory proves neither of the two below, because the scanner must never open
  # one, and this returned "the exclusion stands" for all of them: "I could not
  # look" was being reported as "it is prose".
  #
  # It takes no crafted tree to reach. A tracked path whose worktree copy is not a
  # regular file is dropped from the WORKTREE hop by skipspec, so it reaches the
  # union enumeration ONCE and the staged re-test further down, which runs on a
  # path named twice, never runs for it: committed-and-staged content was excluded
  # from the scan on the strength of a name and a file that is not there. The
  # sandbox makes exactly this shape by bind mounting /dev/null over deny-listed
  # paths, and `git commit` ships the index either way.
  #
  # So the same two questions go to the bytes that WOULD ship. An index matching
  # the base ships nothing new, which keeps every deletion excluded: reading the
  # base copy back would fire on ordinary `rm` of a document. A staged deletion and
  # an unmerged path have no stage 0, so `git show` writes nothing, head2 stays
  # empty and the claim holds for the same reason.
  if [ ! -f "$f" ]; then
    git diff --cached --quiet "$base" -- ":(top,literal)$1" 2>/dev/null && return 0
    mode="$(git ls-files -s -- ":(top,literal)$1" 2>/dev/null)"
    [ "${mode%% *}" = "100755" ] && return 1
    IFS= read -r -n 2 head2 < <(git show ":$1" 2>/dev/null) || head2=""
    case "$head2" in '#!') return 1 ;; esac
    return 0
  fi
  [ -r "$f" ] || return 1
  if [ -x "$f" ]; then
    exec_bit_reliable && return 1
    mode="$(git ls-files -s -- ":(top,literal)$1" 2>/dev/null)"
    [ "${mode%% *}" = "100755" ] && return 1
  fi
  IFS= read -r -n 2 head2 < "$f" 2>/dev/null || head2=""
  case "$head2" in '#!') return 1 ;; esac
  return 0
}

# self_exclude names THIS PLUGIN's own files, so it is ANCHORED rather than
# suffix matched: a suffix let anyone claim the exemption with a filename, and a
# directory merely named `mega-orchestration` let them claim it with an mkdir.
# hooks/risky-logic-gate.md carries why. A prefix has to be PROVEN to hold this
# plugin, by one of exactly two facts:
#
#   1. It is the running installation, resolved physically, and it lies inside
#      the repository under review, so the relative path is ours by construction.
#   2. The repository DECLARES this plugin there in committed content, a
#      .claude-plugin/plugin.json at that prefix whose `name` equals the running
#      installation's. `git show HEAD:`, never the worktree: a pending tree must
#      not mint the anchor that exempts it.
#
# A PATH WITH NO PREFIX IS HELD TO THE SAME STANDARD, through
# self_root_anchored. An exact repository-relative match used to return excluded
# outright, so an unrelated repository with a root `enforcement.toml` went
# unscanned for the price of a filename. The repository has to BE this plugin, or
# say in committed content that it ships it.
plugin_dir="$(cd "$here/.." 2>/dev/null && pwd -P)" || plugin_dir=""
plugin_id=""
[ -z "$plugin_dir" ] ||
  plugin_id="$(jq -r '.name // ""' "$plugin_dir/.claude-plugin/plugin.json" 2>/dev/null)"
plugin_rel=""
root_phys=""
if [ -n "$plugin_dir" ] && [ -n "$root" ]; then
  root_phys="$(cd "$root" 2>/dev/null && pwd -P)" || root_phys=""
  # Physical on both sides, so a symlinked checkout or a symlinked install still
  # recognizes itself instead of failing the prefix test on spelling.
  #
  # The root case is left OUT of plugin_rel deliberately: `${x#"$x"/}` strips
  # nothing, so an installation at the root would set plugin_rel to an absolute
  # path that no repository-relative prefix can ever equal. self_root_anchored owns
  # that case, and it compares the directories rather than a spelling.
  if [ -n "$root_phys" ] && [ "$plugin_dir" != "$root_phys" ]; then
    case "$plugin_dir/" in
      "$root_phys"/*) plugin_rel="${plugin_dir#"$root_phys"/}" ;;
    esac
  fi
fi
# Memoized in two lists rather than recomputed: scan_excluded runs once per changed
# path and once per untracked path, and the committed-manifest read is a git call
# plus a jq call. The lists hold prefixes, of which a repository has one or two.
self_anchor_ok=()
self_anchor_no=()
self_anchored() {
  local pre="$1" x name
  [ -n "$plugin_rel" ] && [ "$pre" = "$plugin_rel" ] && return 0
  for x in ${self_anchor_ok[@]+"${self_anchor_ok[@]}"}; do [ "$x" = "$pre" ] && return 0; done
  for x in ${self_anchor_no[@]+"${self_anchor_no[@]}"}; do [ "$x" = "$pre" ] && return 1; done
  name=""
  [ -n "$plugin_id" ] &&
    name="$(git show "HEAD:$pre/.claude-plugin/plugin.json" 2>/dev/null | jq -r '.name // ""' 2>/dev/null)"
  if [ -n "$plugin_id" ] && [ "$name" = "$plugin_id" ]; then
    self_anchor_ok+=("$pre")
    return 0
  fi
  self_anchor_no+=("$pre")
  return 1
}
# THE EMPTY PREFIX: is the repository under review the one whose files this list
# names? Three ways to prove it, each costing more than picking a filename.
#
#   1. The running installation IS the repository root, compared physically.
#   2. The root carries a COMMITTED .claude-plugin/plugin.json naming this plugin.
#   3. The root carries a COMMITTED .claude-plugin/marketplace.json declaring this
#      plugin at a source prefix that itself proves out by 1 or 2. This is the
#      plugin's own SOURCE repository, where the entries carrying no plugin prefix
#      live (its fixtures sit outside plugins/<name>/, at paths like
#      scripts/tests/enforcement.test.sh). It takes two committed facts, both read
#      with `git show HEAD:`, so a pending tree cannot mint its own exemption.
#
# Memoized in one variable rather than a list, because there is exactly one root.
root_anchor=-1
self_root_anchored() {
  local src
  if [ "$root_anchor" -lt 0 ]; then
    root_anchor=0
    if [ -n "$plugin_dir" ] && [ -n "$root_phys" ] && [ "$plugin_dir" = "$root_phys" ]; then
      root_anchor=1
    elif [ -z "$plugin_id" ]; then
      : # No identity to match, so nothing can be proven and nothing is excluded.
    elif [ "$plugin_id" = "$(git show HEAD:.claude-plugin/plugin.json 2>/dev/null |
                             jq -r '.name // ""' 2>/dev/null)" ]; then
      root_anchor=1
    else
      # `select(.source|type=="string")`, because the marketplace schema also
      # allows a git source object, and an object names no directory in this
      # checkout. The first matching entry decides; a marketplace listing this
      # plugin twice is malformed either way.
      src="$(git show HEAD:.claude-plugin/marketplace.json 2>/dev/null |
        jq -r --arg id "$plugin_id" \
          '[.plugins[]? | select(.name == $id) | select(.source|type == "string") | .source][0] // ""' \
          2>/dev/null)"
      src="${src#./}"
      src="${src%/}"
      # An absolute source, or one climbing out with `..`, points outside the
      # repository and cannot be a prefix of a repository-relative path. An empty
      # one is the root itself, which the two tests above already settled.
      case "$src" in
        ""|/*|..|../*|*/..|*/../*) ;;
        *) self_anchored "$src" && root_anchor=1 ;;
      esac
    fi
  fi
  [ "$root_anchor" -eq 1 ]
}
scan_excluded() {
  local p="$1" g pre
  for g in ${exclude_globs[@]+"${exclude_globs[@]}"}; do
    # Unquoted on purpose: these entries ARE globs, and quoting turns `*.md` into
    # a request for a file literally called `*.md`, which matches nothing and
    # silently reinstates every false positive this key exists to remove.
    # shellcheck disable=SC2254
    case "$p" in $g) ;; *) continue ;; esac
    # One glob matching is enough to reach the claim, and a failed claim settles
    # the path: another glob agreeing that the name looks like prose cannot make
    # an executable file inert.
    exclude_claim_holds "$p" || return 1
    return 0
  done
  for g in ${self_exclude[@]+"${self_exclude[@]}"}; do
    # `continue` rather than `return`, because a later entry may still match this
    # path under a prefix that does prove out.
    if [ "$p" = "$g" ]; then
      self_root_anchored && return 0
      continue
    fi
    case "$p" in */"$g") pre="${p%/"$g"}" ;; *) continue ;; esac
    self_anchored "$pre" && return 0
  done
  return 1
}

# The status is captured separately from the output, because git prints NOTHING
# when it aborts: one tracked path it cannot hash and the whole diff comes back
# empty with status 128. Read as output alone, that empty result is
# indistinguishable from a clean tree, and the guard would pass a diff it never
# computed. The sandbox creates the condition routinely by bind mounting
# /dev/null over deny-listed paths, which turns a tracked file like .env.example
# into a character device.
diff="$(tracked_diff 2>/dev/null)"
rc=$?
# A repository with no commits has no HEAD to diff against, so the staged hop
# always fails there for an ordinary reason. That is not an unreadable tree, but
# it is not an empty one either: the index can already hold staged content, which
# `git ls-files --others` does not report, so calling the diff empty here would
# pass staged risky logic unread. Re-base onto the empty tree instead, which
# every repository has, so the whole index renders as additions.
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
    skipspec+=("${skipped[@]}")
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
#
# `--full-name`, so these names are repository-relative and can be matched against
# exclude_globs and self_exclude, which are written against repository-relative
# paths. Every read below therefore goes through "$root/$f": plain `git ls-files`
# prints names relative to the CURRENT DIRECTORY, so a stop from a subdirectory
# would test one spelling against the rules and open another.
untracked=()
while IFS= read -r -d '' f; do untracked+=("$f"); done \
  < <(git ls-files -z --others --exclude-standard --full-name -- ':/' \
        ${skipspec[@]+"${skipspec[@]}"} 2>/dev/null)
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
  jq -nc --arg detail "$detail" --arg note "$project_note" \
    '{decision:"block", reason:($note + "This gate could not compute the pending diff, so it could not check whether risky auth, billing, payment, or concurrency logic changed. git reported: " + $detail + " Do not treat the tree as clean. Inspect the pending changes yourself. A review receipt cannot clear this block, so either make the path git named readable and stop again, or tell the human plainly that the automated check could not read the diff, which path caused it, and what your own inspection found.")}'
  exit 0
fi

# THE INDEX CAN HIDE A MODIFICATION FROM EVERY READ ABOVE.
#
# `git update-index --assume-unchanged P` tells git to trust the index entry and
# stop stat'ing the worktree, so a later edit to P appears in no `git diff`, no
# `git status`, no untracked listing, and no fingerprint: a tracked change ships
# with no review at all. skip-worktree is NOT the same thing and is not refused
# with it, because sparse checkout sets it on every path outside the cone. `git
# ls-files -v` separates them: LOWERCASE tag is assume-unchanged, 'S' is
# skip-worktree, 's' is both. Content is the trigger, not the bit: a bit on a
# path whose content still matches the index hides nothing at this stop, and
# refusing on the bit alone fires on every stop in a repository that merely uses
# the feature.
#
# ONE index pass serves this check AND the submodule scan below. `-s -v -z`
# delivers the flag, the recorded blob, and the gitlink mode together.
#
# PREFILTER, because this runs on every stop: a NUL-delimited bash read loop over
# a whole index costs about half a second on twenty thousand paths, and almost no
# repository has an index flag or a gitlink. `git ls-files` C-quotes a name
# holding a newline regardless of core.quotePath, so a newline-delimited listing
# puts every entry on one line and grep can decide at C speed whether the precise
# pass is worth it, with no false negatives. The precise pass reads the NUL form
# and stays the only thing that parses a path.
#
# `--full-name` in both listings: without it `git ls-files` prints names relative
# to the CURRENT DIRECTORY and lists only what sits under it, so a stop from a
# subdirectory would test the wrong paths and miss every gitlink above it.
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
index_lines=""
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

    # The assume-unchanged check, stated in full at the index pass above:
    # LOWERCASE tag is assume-unchanged, 'S' is skip-worktree, 's' is both, and
    # content is the trigger rather than the bit.
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
  index_list=""
fi
if [ -n "$hidden" ] || [ "$hidden_rc" -ne 0 ]; then
  # No receipt clears this, and the reason says so: review-diff-id refuses to
  # fingerprint such a tree for the same reason, so sending the agent after a
  # review here would send it after a receipt that cannot be produced.
  detail="the index hides worktree changes at$hidden"
  [ -n "$hidden" ] || detail="this gate could not read the index flags, so it cannot tell whether the index is hiding a worktree change"
  jq -nc --arg detail "$detail" --arg note "$project_note" \
    '{decision:"block", reason:($note + "Pending changes cannot be checked for risky auth, billing, payment, or concurrency logic: " + $detail + ". A path under assume-unchanged is omitted from every diff, from the risky-content scan, and from the review fingerprint, so this gate would report a clean tree over an edit nobody read. Clear the bit with '"'"'git update-index --no-assume-unchanged <path>'"'"' (or --no-skip-worktree) and stop again, then inspect what it exposes. A review receipt cannot clear this block.")}'
  exit 0
fi

[ -n "$diff" ] || [ "${#untracked[@]}" -gt 0 ] || [ "$sub_risky" -eq 1 ] || policy_notice_exit

# TWO scanners over ONE shared marker table: the reduced diff goes through the
# diff scanner, untracked files through the file scanner. The comment rule lives
# in the shared part, so it cannot apply to a tracked change and not to an
# untracked one.
#
# `skip_comments`: a comment cannot authenticate a user or charge a card. Only
# the line's FIRST non-whitespace run counts, so `handler() { /* oauth */ }` is
# still code and still scanned.
#
# COMMENT MARKERS ARE LANGUAGE SPECIFIC, and `#`, `--`, and `;` are executable
# syntax somewhere, so the set is picked from the file EXTENSION and an unknown
# extension scans EVERY line. Every entry owes a proof that its marker cannot be
# executable syntax in that language, and an extension naming more than one
# language proves nothing: it must scan. hooks/risky-logic-gate.md records the
# seven removals that rule cost, the two entries audited and kept (`.yml`/`.yaml`
# and `.css`), and why this is a table rather than a parser. Read it before
# adding an entry.
#
# A MARKER IS NOT A PROOF EITHER: SOME COMMENT-SHAPED LINES EXECUTE. A shebang,
# `//go:build`, an Emacs file-local `eval:`, a server-side include. All are
# written in the file's comment syntax, so they are listed and excepted in
# is_comment. `.sql` is split off the dash set because MySQL and PostgreSQL
# disagree about whether `--payment` is a comment at all.
#
# A MARKER THAT CLOSES ON THE SAME LINE PROVES NOTHING ABOUT THE REST OF IT.
# `/* note */ chargeCard()` executes, so a block opener counts only when the line
# does not also close it. `-->` is out of the table for the reason `*` is.
#
# `*` IS NOT A COMMENT OPENER and is not in the C-family set. It only continues a
# block some earlier line opened, which a one-line-at-a-time scanner cannot know,
# while it dereferences (`*paymentTotal = 0`), starts a generator method, and
# multiplies in exactly the languages that set covers. Scanning a real
# continuation line costs one false positive; missing a dereference costs a risky
# change nobody reads.
markers_prog='
  function markers(p,   e, n, a) {
    n = split(p, a, "/"); p = a[n]
    if (p !~ /\./) return ""
    e = tolower(p); sub(/^.*\./, "", e)
    if (e ~ /^(sh|bash|zsh|ksh|fish|py|rb|pm|jl|nix|ex|exs|cmake|yml|yaml|toml|ini|tf|tfvars|mk)$/) return "#"
    # `.go` gets its own marker: see the "G" branch, where a block comment is
    # compiled C rather than prose.
    if (e == "go") return "G"
    if (e ~ /^(js|mjs|cjs|jsx|ts|tsx|c|h|cc|cpp|cxx|hpp|hh|java|cs|rs|swift|kt|kts|scala|php|dart|proto|sol|zig|groovy|gradle|css|scss|less|mm)$/) return "/"
    # `.sql` gets its own marker: see the "S" branch, where one dash rule cannot
    # cover MySQL and PostgreSQL at once.
    if (e == "sql") return "S"
    if (e ~ /^(hs|lhs|lua|elm|adb|ads|vhd|vhdl)$/) return "-"
    if (e ~ /^(html|htm|xhtml|xml|svg|vue|svelte|md|mdx|markdown)$/) return "<"
    if (e ~ /^(lisp|clj|cljs|cljc|edn|el|scm|ss|nasm)$/) return ";"
    return ""
  }
  function is_comment(l, m,   t, s) {
    if (m == "") return 0
    t = l; sub(/^[ \t]*/, "", t)
    # A SHEBANG IS NEVER A COMMENT, IN ANY LANGUAGE. `#!/usr/bin/payment-wrapper`
    # is the one line that decides what the kernel executes, and the `#` set read
    # it as prose and skipped it. Checked ahead of the marker table because no
    # extension can make it inert, and on the stripped line rather than at column
    # one because scanning an indented `#!` costs one false positive and missing a
    # real one costs the interpreter choice.
    if (t ~ /^#!/) return 0
    # AN EMACS FILE-LOCAL VARIABLES LINE EXECUTES. `-*- eval: (charge-card) -*-`
    # is evaluated when the file is opened, and it is written inside whatever the
    # file comment syntax is, so it reaches every marker set. Ordinary
    # `# -*- coding: utf-8 -*-` headers carry no keyword and cost nothing.
    if (t ~ /-\*-/) return 0
    if (m == "#") return t ~ /^#/
    if (m == "/" || m == "G") {
      if (t ~ /^\/\//) {
        # DIRECTIVES ARE NOT COMMENTS EITHER. These are comment-shaped and
        # semantically live: `//go:build` and `// +build` decide which file
        # compiles at all, `//go:embed` and `//go:generate` decide what is
        # embedded and what runs, `//export` names a cgo entry point, and a
        # TypeScript `/// <reference>` pulls another file into the compilation.
        # `//go:` covers the whole Go pragma family in one test.
        if (t ~ /^\/\/go:/) return 0
        if (t ~ /^\/\/[ \t]*\+build/) return 0
        if (t ~ /^\/\/export[ \t]/) return 0
        if (t ~ /^\/\/\/[ \t]*<reference/) return 0
        return 1
      }
      if (t ~ /^\/\*/) {
        # Opened AND closed here, so what follows the close executes.
        if (substr(t, 3) ~ /\*\//) return 0
        # A GO BLOCK COMMENT CAN BE COMPILED C. The preamble above `import "C"` is
        # C source handed to cgo, so `/* #include <payment.h>` is a directive the C
        # compiler reads, not prose. Every entry here owes a proof that its marker
        # cannot be executable syntax in that language, and Go cannot give one for
        # `/*`, so the opening line scans. Only that line: the continuation lines
        # already scan, because `*` opens nothing and is in no marker set, and a
        # comment that closes on its own line is handled above. In every other
        # C-family language the block really is inert, and scanning the first line of
        # every multi-line comment there would be a broad false positive for nothing.
        return (m != "G")
      }
      return 0
    }
    if (m == "S") {
      if (t !~ /^--/) return 0
      # SQL IS NOT HASKELL, AND THE DIALECTS DISAGREE WITH EACH OTHER. MySQL needs
      # whitespace or a control character after the dashes, so `--payment` there is
      # two unary minus operators applied to a column, while PostgreSQL, SQLite,
      # Oracle and T-SQL read the same text as a comment. Only the form every
      # dialect agrees on is suppressed: dashes followed by whitespace, or a line
      # that is nothing but dashes. Everything else is scanned, because a scanner
      # that has to pick a dialect from a `.sql` extension is guessing.
      return (t ~ /^--[ \t]/) || (t ~ /^--$/)
    }
    if (m == "-") {
      if (t !~ /^--/) return 0
      # The dash run ends at the first symbol character: `---` is a comment,
      # `-->` is an operator Haskell code legally starts a line with.
      s = t; sub(/^-+/, "", s)
      return s !~ /^[!#$%&*+.\/<=>?@\\^|~:]/
    }
    if (m == "<") {
      # A SERVER-SIDE INCLUDE IS A COMMENT ONLY TO THE BROWSER. `<!--#exec cmd=`
      # and `<!--#include` run on the server, and an IE conditional comment
      # `<!--[if IE]>` reveals the markup it wraps. Both are directives wearing
      # comment syntax, so neither is skipped.
      if (t ~ /^<!--#/) return 0
      if (t ~ /^<!--\[if/) return 0
      if (t ~ /^<!--/) return substr(t, 5) !~ /-->/
      return 0
    }
    if (m == ";") return t ~ /^;/
    return 0
  }
  # RECORDS WHAT MATCHED, not just that something did. A block whose reason
  # cannot name the path and the keyword is undiagnosable after the fact: the
  # agent reading it has to guess, and a guess reported as a finding is how a
  # false positive becomes a false security report. `hitword` is global on
  # purpose, read by the END rule of whichever program included this table.
  #
  # match() rather than ~ so RSTART and RLENGTH carry the matched text back. The
  # extra parameters are awk locals, which is why they are declared and never
  # passed.
  function risky_line(l, m,   lo, p) {
    if (skipc && is_comment(l, m)) return 0
    lo = tolower(l)
    p = match(lo, pat)
    if (p == 0) return 0
    hitword = substr(lo, p, RLENGTH)
    return 1
  }
'
# `added_lines_only`: only a line the change ADDS can introduce risky logic.
# Context lines are the tree as it already stood and removed lines are risk going
# away, so scanning all three made every edit near auth code read as an auth
# change.
#
# The header/body split is tracked through `diff --git` and `@@` rather than by
# pattern alone, because an added line whose content starts with `++` renders as
# `+++ ...` and dropping it by shape would drop a real change. That is also why
# the `+++` rule is guarded by the header state rather than by its shape alone.
# Tracking the header is what supplies the marker table with a filename, since
# `+++ b/<path>` names the file the following hunk belongs to.
#
# A BINARY PATCH IS A HIT, NOT AN EMPTY SCAN. A `GIT binary patch` section carries
# no `@@` and no text, so a changed executable, archive, or model file produced
# zero scanned lines and the tree read clean unless some other file tripped the
# gate. An empty textual scan of a binary is not evidence of safety. The path is
# reported instead and the caller blocks naming it, through the ordinary
# receipt-clearable route: the review package covers the same binary delta, so an
# independent review can legitimately clear it.
diff_scan_prog=$markers_prog'
  # The path a binary block is reported under. `diff --git a/X b/X` has no
  # unambiguous separator once a name holds a space, and git quotes such a name,
  # so an unparseable header falls back to the whole remainder: the reason has to
  # name something the reader can find, which is a weaker requirement than
  # recovering the exact name.
  function dgpath(l,   s, i) {
    s = substr(l, 12)
    i = index(s, " b/")
    if (i > 0) return substr(s, i + 3)
    return s
  }
  /^diff --git / {
    inhdr = 1; cls = ""; dg = dgpath($0)
    if (!addedonly && risky_line($0, cls)) { hit = 1; exit }
    next
  }
  inhdr && /^GIT binary patch$/ { if (binary == "") binary = dg; next }
  inhdr && /^Binary files /     { if (binary == "") binary = dg; next }
  inhdr && /^\+\+\+ / {
    cls = markers(substr($0, 5))
    if (!addedonly && risky_line($0, cls)) { hit = 1; exit }
    next
  }
  /^@@/ { inhdr = 0; if (!addedonly && risky_line($0, cls)) { hit = 1; exit } next }
  inhdr { if (!addedonly && risky_line($0, cls)) { hit = 1; exit } next }
  {
    if (addedonly) {
      if (substr($0, 1, 1) == "+" && risky_line(substr($0, 2), cls)) { hit = 1; exit }
    } else if (risky_line($0, cls)) { hit = 1; exit }
  }
  END {
    # The path comes from the last `diff --git` header seen, which is the file
    # the matching hunk belongs to. A hit inside a header line has the same dg.
    if (hit) { print (dg == "" ? "a path git did not name" : dg) "\t" hitword; exit 0 }
    if (binary != "") { print binary; exit 2 }
    exit 1
  }
'
# The file scanner takes the filename from awk itself, so one table serves both
# callers. Reading stdin leaves FILENAME empty, which resolves to no markers and
# therefore scans every line: that is the path the untracked NAME scan takes, and
# a path is not a line of source.
file_scan_prog=$markers_prog'
  FNR == 1 { cls = markers(FILENAME) }
  { if (risky_line($0, cls)) { hit = 1; hitfile = FILENAME; hitline = FNR; hitname = $0; exit } }
  # STDIN IS THE UNTRACKED NAME SCAN, and there the matched LINE is itself the
  # path, so the line is the location. gawk sets FILENAME to "-" for stdin, not
  # to "", so testing only for empty left every name-scan hit reporting "-:1"
  # and never the path that matched. Both spellings are handled because the
  # value is unspecified by POSIX and differs between implementations.
  END {
    if (!hit) exit 1
    if (hitfile == "" || hitfile == "-") print hitname "\t" hitword
    else print hitfile ":" hitline "\t" hitword
    exit 0
  }
'
# STDOUT IS EVIDENCE, NOT OUTPUT. Every caller captures it. Letting it reach the
# hook stdout unread would splice a path into the JSON the harness parses.
scan_lines() { awk -v pat="$risky" -v skipc="$1" "$file_scan_prog"; }

# A NUL byte is the test git itself uses to call content binary, so this needs no
# new dependency and no extension list. An extension list would let a rename
# decide what goes unread, the same hole the self_exclude anchoring closed.
#
# A BOUNDED PREFIX IS NOT A CLASSIFICATION. Reading one 8192 character window and
# calling everything without a NUL text let a file whose first NUL sits at byte
# 8193 pass as source in BOTH legs: git sniffs only its own first 8000 bytes and
# renders a text-shaped diff, and bash strips the later NUL out of the captured
# diff, so awk saw a keyword-free file. Both reproduced. The scan runs to the end
# now, and where it cannot finish it says UNDECIDED rather than text, because
# text is the answer that skips.
#
# Status 0 holds a NUL, 1 does not, 2 could not be established. Every caller
# treats 2 like 0: "not known to be text" is the only honest reading.
#
# BUILTINS ONLY, no pipeline. This runs once per untracked path and once per
# changed tracked path; a head/tr/wc sniff costs three processes per file and
# blew the untracked-cap budget at 1424ms against a 1000ms limit, which every
# version of this function has had to meet. `read -d ''` stops at the first NUL
# and reports success when it found that delimiter; `-n` takes one window at a
# time so a multi-gigabyte artifact is never slurped whole. The two success cases
# are told apart by LENGTH, short means the NUL ended the read, because command
# substitution cannot carry the byte back: bash drops NUL from strings.
#
# $2 windows bounds TIME, not content: the builtin reads about a mebibyte every
# 15ms here, so 16 windows is one mebibyte per file. A larger file with no NUL in
# the part read is undecided, which gates.
nul_window=65536
nul_limit=16
nul_scan() {
  # LC_ALL=C SO THE WINDOW IS BYTES. `read -n` counts CHARACTERS, so under a UTF-8
  # locale a 65536 character window is up to four times that many bytes and the
  # mebibyte bound above is per character: the time bound it exists to enforce moves
  # with the content of the file being read, which is the one thing a work bound
  # must not do. Measured at 400000 bytes of two-byte characters: six windows under
  # C, three under en_US.utf8. NUL detection is unaffected, because `-d ''` is a NUL
  # BYTE in every locale, and C also stops an invalid multibyte sequence from ending
  # a read early. `local` so the setting lasts exactly this call: bash restores the
  # previous value, set or unset, on return.
  local LC_ALL=C
  local chunk="" seen=0
  # Unreadable is undecided, never text. The name is chosen by whoever adds the
  # file, so a mode that stops the read must not also stop the gate.
  [ -r "$1" ] || return 2
  # The whole loop and the probe read one shared descriptor, which is why they sit
  # inside a redirected group rather than each carrying their own `< "$1"`: the
  # probe has to continue where the last window stopped.
  {
    while IFS= read -r -d '' -n "$nul_window" chunk; do
      [ "${#chunk}" -lt "$nul_window" ] && return 0
      seen=$((seen + 1))
      # THE BUDGET IS SPENT, THE FILE IS NOT NECESSARILY BIGGER THAN IT. Returning
      # undecided here read every byte of an exactly-$2-window file, found no NUL,
      # and then blocked it anyway, although the comment above promises undecided
      # only ABOVE the bound. One more byte tells the two apart, and it is the only
      # extra work this costs. Empty on success means the NUL delimiter ended the
      # read; one byte means content continues past the bound and nothing here
      # decided it; failure means end of file, so every byte was read and none was
      # a NUL.
      if [ "$seen" -ge "$2" ]; then
        if IFS= read -r -d '' -n 1 chunk; then
          [ -n "$chunk" ] && return 2
          return 0
        fi
        return 1
      fi
    done
    return 1
  } < "$1"
}

# AN UNSCANNABLE CHANGE IS ANNOUNCED, NOT BLOCKED, UNLESS ITS PATH SAYS OTHERWISE.
#
# NOT SCANNED is a FACT and goes out on the non-blocking systemMessage, the same
# channel a pending policy edit uses. RISKY is a CLAIM needing evidence, and the
# only evidence left when the bytes cannot be read is the path, so a path
# matching the keyword list still blocks: a changed payment_processor.bin is a
# different proposition from a changed favicon.png.
#
# Silence is never the outcome. When something else blocks, these sentences ride
# in the block preamble; when nothing else does, they go out as a notice. Both
# halves are load-bearing and hooks/risky-logic-gate.md records why: blocking
# everything unscannable fired on adding a favicon, and blocking nothing let a
# staged compiled artifact buy a clean bill of health.
hit="$sub_risky"
if [ "$hit" -eq 0 ] && [ -n "$diff" ]; then
  # Excluded paths are dropped from a SECOND diff, never from the one computed
  # above. git spells the exclusion itself, with `literal` so a name holding a `*`
  # excludes itself and nothing else, and the diff everything downstream reads
  # stays whole. Building the drop list needs the path enumeration git already
  # has; parsing names back out of the diff text would have to undo core.quotePath
  # first, and a name it failed to unquote would silently drop the wrong file.
  scan_diff="$diff"
  drop=()
  # THE RENDERED DIFF CANNOT CLASSIFY THE FILE IT RENDERS. git calls a blob
  # binary on its own first 8000 bytes, so a file whose first NUL sits past that
  # renders as ordinary added lines with no `GIT binary patch` section at all, and
  # the capture above is a bash string, which drops every NUL git did write. The
  # scanner then reads a keyword-free text file and the tree passes: staging the
  # artifact was the whole exploit. So the classification is taken from the BYTES
  # on disk, through the enumeration this loop already runs, and a path that
  # cannot be classified counts the same as one that is binary.
  #
  # AFTER the exclusion, matching the untracked leg: an excluded path is dropped
  # from the scanned diff, so a binary under an excluded glob is reported in
  # neither leg and the two legs agree about the same file.
  #
  # BOTH HOPS ARE ENUMERATED, because the diff being reduced has both. Naming only
  # the worktree hop would leave a staged-only path out of the drop list, so its
  # excluded prose would be rescanned and its bytes never sniffed at all.
  #
  # `sort -z` and NOT `-zu`, because the duplicate carries information: a path named
  # by both hops is one whose index content differs from its worktree content. The
  # first occurrence does the ordinary work and the second classifies the INDEX
  # bytes, which is what a plain `git commit` ships. Sorted, so the two occurrences
  # are adjacent and one comparison finds them. The drop list still gets one operand
  # per path, because a duplicate operand spends the execve argument budget the
  # fallback below exists to survive.
  tracked_binary=""
  tracked_unclassified=""
  staged_blob=""
  prev=""
  prev_dropped=0
  while IFS= read -r -d '' p; do
    if [ "$p" = "$prev" ]; then
      # THE WORKTREE COPY IS NOT THE STAGED COPY HERE. `git add` an artifact and
      # then delete or rewrite the file and the sniff below reads bytes that are no
      # longer what would ship: a late-NUL binary staged and then removed from the
      # worktree renders as ordinary added lines, sniffs nothing on disk because
      # there is nothing on disk, and passes as text. A path named by ONE hop needs
      # no second read: differing only in the index means the worktree copy IS the
      # index content, and differing only in the worktree means it was never staged.
      #
      # THE SHORT-CIRCUIT IS THE RISKY PATH, NOT THE FIRST BINARY. Stopping at the
      # first unscannable file was free while every one of them blocked. Now that
      # only a risky-named one does, stopping there would let `git add favicon.png
      # payment_processor.bin` decide by enumeration order which of the two the gate
      # ever looked at. Sniffing continues until there is nothing further to learn,
      # matching the untracked leg, which has never short-circuited.
      [ -n "$unscannable_risky" ] && [ "$prev_dropped" -eq 0 ] && continue
      [ -n "$staged_blob" ] || staged_blob="$(mktemp 2>/dev/null)" || staged_blob=""
      # Materialized, because nul_scan reads windows from a regular file: over a
      # pipe a short read is indistinguishable from the NUL that ends one, which is
      # the difference between binary and text. Unreadable staged bytes are
      # undecided rather than text, for the reason every other leg gives.
      if [ -z "$staged_blob" ] || ! git show ":$p" > "$staged_blob" 2>/dev/null; then
        # AN UNMERGED PATH HAS NO STAGE 0 AND NOTHING TO SHIP. `git commit` refuses
        # while a conflict is open, so there is no staged content here to classify,
        # and the worktree copy carrying the conflict markers was already sniffed on
        # the first occurrence. Calling it unclassifiable would block every stop in
        # the middle of a merge, and a gate that fires on ordinary work is a gate
        # sessions learn to route around. Anything else is content this gate could
        # not read, and unread is not text.
        [ -n "$(git ls-files -u -- ":(top,literal)$p" 2>/dev/null)" ] && continue
        [ -n "$tracked_unclassified" ] || tracked_unclassified="$p"
        continue
      fi
      if [ "$prev_dropped" -eq 1 ]; then
        # THE EXCLUSION CLAIM WAS TESTED AGAINST THE WORKTREE COPY TOO. `*.md` is
        # excluded because prose does not execute, and exclude_claim_holds checks
        # that per file against the bytes on disk. Stage an executable shebang
        # payload as billing.md, restore the worktree copy to inert prose, and the
        # claim held over a file that no longer had anything to do with what would
        # ship. The index mode and the staged first two bytes answer the same two
        # questions about the copy that WOULD ship, and either one contradicting the
        # claim revokes the exclusion for this path.
        #
        # This leg is for a REGULAR worktree copy that answered, inertly, while the
        # index held something else. A copy that cannot answer at all never gets
        # here, because such a path is named ONCE: exclude_claim_holds puts the same
        # two questions to the index itself for those.
        #
        # Revoked by REBUILDING the list rather than unsetting an element: `unset`
        # leaves a sparse array whose ${#a[@]} is a count and not a last index, so
        # the next revocation would drop the wrong operand. Revocations are rare and
        # the list is short enough that the fallback below survives it.
        staged_mode="$(git ls-files -s -- ":(top,literal)$p" 2>/dev/null)"
        staged_mode="${staged_mode%% *}"
        staged_head=""
        IFS= read -r -n 2 staged_head < "$staged_blob" 2>/dev/null || staged_head=""
        if [ "$staged_mode" = "100755" ] || [ "$staged_head" = '#!' ]; then
          keep=()
          for spec in ${drop[@]+"${drop[@]}"}; do
            [ "$spec" = ":(top,exclude,literal)$p" ] && continue
            keep+=("$spec")
          done
          drop=(${keep[@]+"${keep[@]}"})
          prev_dropped=0
        else
          continue
        fi
      fi
      [ -n "$unscannable_risky" ] && continue
      nul_scan "$staged_blob" "$nul_limit"
      # The first of each kind is kept for the message, and every one of them has
      # its path tested, because the path is the only risk signal these bytes leave.
      case "$?" in
        0) [ -n "$tracked_binary" ] || tracked_binary="$p"
           path_is_risky "$p" && unscannable_risky="$p" ;;
        2) [ -n "$tracked_unclassified" ] || tracked_unclassified="$p"
           path_is_risky "$p" && unscannable_risky="$p" ;;
      esac
      continue
    fi
    prev="$p"
    prev_dropped=0
    if scan_excluded "$p"; then
      drop+=(":(top,exclude,literal)$p")
      prev_dropped=1
      continue
    fi
    [ -n "$unscannable_risky" ] && continue
    # A symlink diffs as its target string, so it is not content this gate can read.
    [ -L "$root/$p" ] && continue
    if [ ! -f "$root/$p" ]; then
      # NO REGULAR WORKTREE COPY, AND THE INDEX MAY STILL SHIP BYTES. The staged
      # read below only runs for a path BOTH hops name, on the reasoning that a path
      # named once differs in one place and the worktree copy is therefore the index
      # content. That reasoning fails when the worktree hop cannot report the path at
      # all: a tracked path whose worktree copy is a fifo or a character device is
      # dropped from that hop by skipspec, so a staged unscannable blob under it is
      # named ONCE and classified from a worktree copy that does not exist. The
      # sandbox creates exactly that shape by bind mounting /dev/null over
      # deny-listed paths, and `git commit` ships the index either way.
      #
      # A DELETION STILL CLASSIFIES NOTHING. Staged, the index holds no stage 0 and
      # `git show` fails; unstaged, the index still matches the base and the
      # `--cached` test is quiet, so neither reaches the sniff. Removed content
      # executes nowhere, and announcing it would fire on every ordinary `rm` of a
      # binary.
      git diff --cached --quiet "$base" -- ":(top,literal)$p" 2>/dev/null && continue
      [ -n "$staged_blob" ] || staged_blob="$(mktemp 2>/dev/null)" || staged_blob=""
      # A `git show` failure here is the deletion case again, either staged or
      # unmerged: no stage 0 exists, so there is nothing to classify and nothing to
      # say.
      if [ -z "$staged_blob" ] || ! git show ":$p" > "$staged_blob" 2>/dev/null; then
        continue
      fi
      nul_scan "$staged_blob" "$nul_limit"
      case "$?" in
        0) [ -n "$tracked_binary" ] || tracked_binary="$p"
           path_is_risky "$p" && unscannable_risky="$p" ;;
        2) [ -n "$tracked_unclassified" ] || tracked_unclassified="$p"
           path_is_risky "$p" && unscannable_risky="$p" ;;
      esac
      continue
    fi
    nul_scan "$root/$p" "$nul_limit"
    case "$?" in
      0) [ -n "$tracked_binary" ] || tracked_binary="$p"
         path_is_risky "$p" && unscannable_risky="$p" ;;
      2) [ -n "$tracked_unclassified" ] || tracked_unclassified="$p"
         path_is_risky "$p" && unscannable_risky="$p" ;;
    esac
  done < <({ git diff --cached "$base" --name-only -z -- ':/' 2>/dev/null
             git diff --name-only -z -- ':/' ${skipspec[@]+"${skipspec[@]}"} 2>/dev/null
           } | LC_ALL=C sort -z)
  [ -z "$staged_blob" ] || rm -f "$staged_blob"
  staged_blob=""
  if [ "${#drop[@]}" -gt 0 ]; then
    # THE STATUS DECIDES, not the output, for the reason the first diff already
    # states: git prints nothing when it aborts, so an unchecked capture turns any
    # failure into an empty scan the gate reads as a clean tree. This list is the
    # one place that failure is reachable without a hostile tree, because it grows
    # one operand per excluded changed path with no bound: a few thousand long
    # paths pass the execve argument limit and the command never runs at all.
    # Falling back to the UNREDUCED diff is the conservative direction. It scans
    # the excluded prose again, which costs false positives, where an empty scan
    # costs a risky change nobody reads.
    #
    # Through tracked_diff and not a fourth hand-written command, so the reduced
    # rescan is the same TWO HOPS and the same flags as the diff it replaces. A
    # rescan that dropped the staged hop would undo the whole union: an excluded
    # path anywhere in the tree would silently return the gate to reading the
    # worktree alone. `--binary` rides along with it, so a changed binary renders
    # the literal `GIT binary patch` the scanner detects rather than the localized
    # "Binary files ... differ" sentence git writes without it, and a gate must not
    # depend on the reader's language for what it can detect.
    dropspec=("${drop[@]}")
    reduced="$(tracked_diff 2>/dev/null)" && scan_diff="$reduced"
    dropspec=()
  fi
  scan_out="$(printf '%s\n' "$scan_diff" |
    awk -v pat="$risky" -v skipc="$skip_comments" -v addedonly="$added_only" "$diff_scan_prog")"
  case "$?" in
    0) hit=1; hit_evidence="$scan_out" ;;
    2) # Named, because "something binary changed" is not a finding a reader can
       # act on and the path is the only part of a binary this gate can report. It
       # is also the only risk signal such a file leaves, so it is matched against
       # the keyword list rather than assumed risky.
       binary_path="$scan_out"
       [ -n "$binary_path" ] || binary_path="a path git did not name"
       path_is_risky "$binary_path" && unscannable_risky="$binary_path"
       binary_note="A binary file changed and cannot be scanned for risky logic as text: $binary_path. An empty textual scan of a binary is not evidence of safety. " ;;
  esac
  # Said even when the keyword scan already fired: the reader is deciding what to
  # look at, and "a changed file git showed me as text is not text" is a separate
  # fact from whichever line matched.
  if [ -n "$tracked_binary" ]; then
    binary_note="${binary_note}A changed tracked file holds NUL bytes, so it is binary whatever git rendered: $tracked_binary. git decides on the first 8000 bytes of a blob, so a binary carrying a text prefix diffs as ordinary lines and its empty textual scan says nothing. "
  elif [ -n "$tracked_unclassified" ]; then
    binary_note="${binary_note}A changed tracked file could not be classified as text or binary within the bytes this gate reads: $tracked_unclassified. Not known to be text is not the same as read. "
  fi
fi
if [ "$hit" -eq 0 ] && [ "${#untracked[@]}" -gt 0 ]; then
  scan_paths=()
  untracked_unreadable=""
  for f in "${untracked[@]}"; do
    # UNREADABLE IS DECIDED BEFORE EXCLUDED. An exclusion is a statement about
    # content the gate CAN read. A regular file it cannot open is not known to be
    # prose, only known to be unread, and the name is chosen by whoever adds the
    # file: ordering the exclusion first let `notes.md` at mode 000 pass while the
    # same bytes as `notes.go` were scanned.
    #
    # WHAT IS DECIDED HERE IS "WAS THIS READ", NOT "IS THIS RISKY". Separating
    # them is what lets the early position survive the announce-not-block rule
    # above: recorded here, announced by the note below, promoted to a block only
    # by `unscannable_risky` when the path names a risky category.
    #
    # SILENCE IS STILL NEVER THE OUTCOME. The scan loop below skips a file it
    # cannot open, so without this record the file would reach neither the scanner
    # nor the reader. The note is emitted unconditionally and names the path.
    if [ -f "$root/$f" ] && [ ! -r "$root/$f" ]; then
      [ -n "$untracked_unreadable" ] || untracked_unreadable="$f"
      path_is_risky "$f" && unscannable_risky="$f"
    fi
    scan_excluded "$f" || scan_paths+=("$f")
  done
  # The NAMES are scanned with the comment rule off: a path called
  # payment_handler.go says what it is before anything is read out of it, and a
  # path is not a line of source.
  if [ "${#scan_paths[@]}" -gt 0 ]; then
    scan_out="$(printf '%s\n' "${scan_paths[@]}" | scan_lines 0)" && {
      hit=1
      hit_evidence="$scan_out"
    }
  fi
  # EVERY remaining regular untracked file is opened. Stopping at the first fifty
  # paths let path fifty-one carry risky content past the scan with nothing said,
  # and a silent bound on a security scan is a hole wherever it is drawn. The
  # batching is only about the execve argument limit, not about how much gets
  # read, and the loop stops early once something has already been found.
  #
  # `[ -f ]` is the filter that matters: it keeps the scanner off a fifo, which
  # would block forever and hang the Stop hook. Symlinks to regular files still
  # read, which is what git would diff anyway.
  #
  # The operands are ABSOLUTE, which is not cosmetic: awk reads an operand of the
  # form `name=value` as a variable assignment and never opens it, so an untracked
  # file called `a=b.go` would go unscanned under a bare relative name. `/` is not
  # valid in an awk identifier, so an absolute path can only be a filename.
  # AN UNTRACKED BINARY IS A HIT, exactly as a binary in the diff is. The diff leg
  # already treats a `GIT binary patch` as risky by construction, but the same
  # bytes left unstaged reached awk as text, matched nothing, and passed: `git add`
  # was the whole difference between a block naming the path and silence. Dropping
  # a built artifact into a worktree without staging it is the ordinary agent
  # workflow, so this needs no adversarial step to reach.
  untracked_binary=""
  untracked_unclassified=""
  scan_files=()
  for f in ${scan_paths[@]+"${scan_paths[@]}"}; do
    [ -f "$root/$f" ] || continue
    # A regular file the scan cannot open is content the gate has not read, and
    # that is the same silence as skipping it: the scanner writes an error nobody
    # sees and reports no match, so unknown content reads as benign. The
    # enumeration above already recorded it and tested its path, ahead of the
    # exclusion so the rule cannot be narrowed by a filename; dropping it here only
    # keeps the scanner off a file it cannot open. nul_scan would also refuse it,
    # but as `untracked_unclassified`, which would tell the reader the bytes were
    # read and found undecided when they were never read at all.
    [ -r "$root/$f" ] || continue
    # After the exclusions, matching the diff leg: an excluded path is dropped
    # from the scanned diff there, so a binary under an excluded glob is reported
    # in neither leg. Sniffing before the exclusion would gate every image added
    # under docs/ and make the two legs disagree about the same file.
    nul_scan "$root/$f" "$nul_limit"
    # The path is tested here as well as by the NAME scan above, which already
    # covers these same paths. Stating the rule where the classification happens is
    # what keeps it true of unscannable files specifically: a later narrowing of the
    # name scan must not quietly take the only signal these bytes leave with it.
    case "$?" in
      0) [ -n "$untracked_binary" ] || untracked_binary="$f"
         path_is_risky "$f" && unscannable_risky="$f"
         continue ;;
      2) [ -n "$untracked_unclassified" ] || untracked_unclassified="$f"
         path_is_risky "$f" && unscannable_risky="$f"
         continue ;;
    esac
    scan_files+=("$root/$f")
  done
  # Named, for the reason the diff leg names its own: "something binary is here"
  # is not a finding a reader can act on, and the path is the only part of a
  # binary this gate can report. Appended rather than assigned, because a tracked
  # binary change and an untracked binary addition are two separate facts and the
  # reader needs both.
  # THE UNREADABLE FILE IS NAMED OUT LOUD, and this line is what keeps the round-3
  # hole shut now that the enumeration above only records. Unconditional, and
  # separate from the two below, because "the gate could not open it" and "the gate
  # opened it and the bytes are not text" are different facts about how far the scan
  # got, and the reader deciding what to look at needs the one that applies.
  [ -z "$untracked_unreadable" ] || binary_note="${binary_note}An untracked file could not be opened at all, so none of its content was scanned: $untracked_unreadable. Unread is not the same as absent, and a mode is chosen by whoever adds the file. "
  [ -z "$untracked_binary" ] || binary_note="${binary_note}An untracked binary file was added and cannot be scanned for risky logic as text: $untracked_binary. An empty textual scan of a binary is not evidence of safety. "
  [ -z "$untracked_unclassified" ] || binary_note="${binary_note}An untracked file could not be classified as text or binary within the bytes this gate reads: $untracked_unclassified. Not known to be text is not the same as read. "
  i=0
  while [ "$hit" -eq 0 ] && [ "$i" -lt "${#scan_files[@]}" ]; do
    scan_out="$(awk -v pat="$risky" -v skipc="$skip_comments" "$file_scan_prog" \
      "${scan_files[@]:i:200}" 2>/dev/null)" && {
      hit=1
      hit_evidence="$scan_out"
    }
    i=$((i + 200))
  done
fi
# THE ONE PLACE UNSCANNABLE CONTENT BECOMES A BLOCK, said once after both legs so
# the rule cannot drift between them and the reader is told which fact promoted an
# announcement into a finding. Everything above only records; nothing above decides.
if [ -n "$unscannable_risky" ]; then
  hit=1
  binary_note="${binary_note}The path $unscannable_risky names one of the risky categories this gate watches, and its content could not be scanned, so that change is treated as risky by construction rather than announced. "
fi
[ "$hit" -eq 1 ] || policy_notice_exit

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
    # A receipt clears the FINDING, not the policy edit. The reviewer is a
    # delegate model, and the thing a pending layer edit needs is a human read.
    policy_notice_exit
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

# Everything the reader has to know BEFORE the finding itself: a rules file that
# could not be read, a pending policy edit this gate refused to honor, a binary it
# could not scan, and a receipt it declined. Each one changes how the block below
# should be read, so none of them may be left to a comment nobody sees.
# WHAT MATCHED, named before the remedy. Without it this reason states only a
# CATEGORY, and the pending change may have nothing to do with that category: the
# agent receiving the block cannot tell a true fire from a false one, so it
# guesses. In the 2026-08-11 transcript audit three of twelve sampled fires were
# guessed wrong in session, one of them reported to the human as a live
# credential exposure that did not exist.
#
# One keyword and one location is the entire trigger. Saying so is also what
# makes the rule tunable: a false fire that names its own cause can be filed
# against the keyword list, and until now none of them could be.
evidence_note=""
if [ -n "$hit_evidence" ]; then
  # Split at the LAST tab, not the first: a path may contain one and a keyword
  # from the list never can, so the final field is unambiguously the keyword.
  evidence_note="What matched: the keyword '${hit_evidence##*$'\t'}' at ${hit_evidence%$'\t'*}. That single match is the whole trigger. If that line is not auth, billing, payment, or concurrency logic, this is a false positive: say so plainly to the human instead of inventing a risk that fits the category, and do not report a finding you have not confirmed by reading the line. "
fi
preamble="$rules_note$project_note$binary_note$evidence_note"

if [ "$reachable" -lt 2 ]; then
  jq -nc --arg legacy "$preamble$legacy" \
    '{decision:"block", reason:($legacy + "Risky auth, billing, payment, or concurrency logic changed, and no independent reviewer is reachable: fewer than two delegate vendors have an installed CLI, so a different-vendor review cannot be resolved on this machine. Do not silently ship it. Summarize the risky change and its blast radius for the human and get an explicit go-ahead, or install a second vendor CLI and re-run the independent pass. Say plainly that the automated cross-vendor check did not run.")}'
  exit 0
fi

jq -nc --arg launcher "$launcher" --arg legacy "$preamble$legacy" \
  '{decision:"block", reason:($legacy + "Risky auth, billing, payment, or concurrency logic changed without a current independent approval receipt. Run " + $launcher + " --role verify --author-vendor <artifact-author-vendor> --artifact worktree --claim <claim>. The launcher resolves a different-vendor reviewer and binds its verdict to the pending tree git reports, plus the base commit it applies to. Ignored paths and content git does not surface in a diff are outside that binding. Unrelated delegate calls and stale receipts do not satisfy this gate.")}'
exit 0
