# Design red flags

Use this reference to screen competing interface or module sketches. A red flag
is a reason to revise or reject a sketch, not an automatic veto.

## Start from the caller

Before comparing internal shapes, write two or three realistic call sites for
each sketch: what the caller imports, calls, and gets back. When a sketch and
its call sites disagree, fix the sketch. The caller's experience is the
specification the types serve.

## Shallow module

The interface is large relative to the complexity it hides. Signs:

- callers coordinate several methods to complete one operation;
- public options expose internal stages or implementation choices;
- learning the interface does not spare the caller from learning the
  implementation.

Prefer a small interface over substantial behavior. A deep call chain is not a
deep module; it spreads understanding across layers.

## Information leakage

Several modules depend on the same internal decision, such as a representation,
policy, storage schema, or wire format, so changing it needs coordinated edits.
Parse external data into domain types at the boundary and keep transport,
framework, and storage details private.

## Temporal decomposition

Modules follow execution order, such as load, validate, transform, and save,
instead of the knowledge they own, so one representation and its invariants
repeat across boundaries. Group code by the decisions it protects.

## Pass-through layer

A method forwards the same arguments to another method of the same shape and
hides nothing. Remove it, or keep it only when it adds policy, adaptation, or a
distinct abstraction.

## Foundation and scope

Before broadening a design, identify the processed entity, when it exists and
changes, the trigger, who owns the result, and current exclusions. Distinguish
direct requirements from engineering inferences. For a material choice, compare
the simplest existing-entity path with the proposed extension. Prefer the
existing path when it satisfies current required behavior; justify an extension
with a concrete current scenario it must cover. Seek agreement only for a
material product behavior choice left open by primary requirements, with the
tradeoff explained; do not reopen settled behavior or elevate routine
implementation details to product decisions. Broad plan approval does not
settle an unexplained choice. Keep this check proportional and inline.
