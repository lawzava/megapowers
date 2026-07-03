# Skill authoring artifacts (not shipped)

Development-only material kept out of the installed skill tree. These are the
authoring byproducts of the skills under `plugins/`: creation logs and the
pressure-scenario fixtures used to test a skill's guidance against a
no-guidance control (see `writing-skills/testing-skills-with-subagents.md`).

They live here, not under `plugins/*/skills/*/`, so they are not installed with
a plugin and do not read as runtime reference an agent might load. Nothing
outside this directory references them. They are candidate fixtures for a future
skill-eval harness.
