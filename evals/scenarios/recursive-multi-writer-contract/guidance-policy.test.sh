#!/usr/bin/env bash
set -u

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
parser="$SCRIPT_DIR/guidance-policy.awk"
passed=0
failed=0

accepts() {
  name=$1
  fixture=$2
  if printf '%s\n' "$fixture" | awk -f "$parser"; then
    printf 'ok: %s\n' "$name"
    passed=$((passed + 1))
  else
    printf 'not ok: %s: valid policy rejected\n' "$name"
    failed=$((failed + 1))
  fi
}

rejects() {
  name=$1
  fixture=$2
  if printf '%s\n' "$fixture" | awk -f "$parser"; then
    printf 'not ok: %s: invalid policy accepted\n' "$name"
    failed=$((failed + 1))
  else
    printf 'ok: %s\n' "$name"
    passed=$((passed + 1))
  fi
}

valid='<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=forbidden
max_task_components_beneath_root=5
-->'

accepts exact-valid-block "$valid"

rejects wrong-writer-slot-release '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=slot-number-allowed
agent_teams=forbidden
max_task_components_beneath_root=5
-->'
rejects wrong-agent-teams '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=allowed
max_task_components_beneath_root=5
-->'
rejects wrong-max-task-components '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=forbidden
max_task_components_beneath_root=6
-->'

rejects missing-writer-slot-release '<!-- megapowers-recursive-sdd-policy:v1
agent_teams=forbidden
max_task_components_beneath_root=5
-->'
rejects missing-agent-teams '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
max_task_components_beneath_root=5
-->'
rejects missing-max-task-components '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=forbidden
-->'

rejects duplicate-writer-slot-release '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
writer_slot_release=exact-token-required
agent_teams=forbidden
max_task_components_beneath_root=5
-->'
rejects duplicate-agent-teams '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=forbidden
agent_teams=forbidden
max_task_components_beneath_root=5
-->'
rejects duplicate-max-task-components '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=forbidden
max_task_components_beneath_root=5
max_task_components_beneath_root=5
-->'

rejects unknown-field '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=forbidden
max_task_components_beneath_root=5
coordinator_fallback=forbidden
-->'

rejects duplicate-blocks "$valid
$valid"
rejects missing-start 'writer_slot_release=exact-token-required
agent_teams=forbidden
max_task_components_beneath_root=5
-->'
rejects missing-end '<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=forbidden
max_task_components_beneath_root=5'
rejects nested-block '<!-- megapowers-recursive-sdd-policy:v1
<!-- megapowers-recursive-sdd-policy:v1
writer_slot_release=exact-token-required
agent_teams=forbidden
max_task_components_beneath_root=5
-->
-->'

accepts outside-content-ignored "Prose before the policy block.
writer_slot_release=slot-number-allowed
$valid
agent_teams=allowed
max_task_components_beneath_root=99
Prose after the policy block."

printf '== guidance policy fixtures: %s passed, %s failed ==\n' "$passed" "$failed"
test "$failed" -eq 0
