BEGIN {
  start_marker = "<!-- megapowers-recursive-sdd-policy:v1"
  end_marker = "-->"
}

$0 == start_marker {
  start_count++
  if (in_block)
    invalid = 1
  in_block = 1
  next
}

$0 == end_marker {
  end_count++
  if (!in_block)
    invalid = 1
  in_block = 0
  next
}

in_block {
  if ($0 == "writer_slot_release=exact-token-required")
    writer_slot_release++
  else if ($0 == "agent_teams=forbidden")
    agent_teams++
  else if ($0 == "max_task_components_beneath_root=5")
    max_task_components++
  else
    invalid = 1
}

END {
  valid = start_count == 1 && end_count == 1 && !in_block && !invalid &&
          writer_slot_release == 1 && agent_teams == 1 &&
          max_task_components == 1
  exit !valid
}
