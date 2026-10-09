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

## Split ownership

Two modules each hold a copy of the same state and keep it in sync. Give the
state one owner and let everything else read from it.

## Two ways to do one task

Two public paths do the same job, so callers and later contributors pick one
at random and the paths drift. Keep one and delete or redirect the other.

## Importable internals

Internal helpers are reachable from outside the module, so callers bypass the
interface. Make the import fail through visibility, package layout, or a lint.

## Hand-synced lists

The same list, such as a registry, enum, route table, or set of names, is
maintained in several places by hand. Derive the copies from one source, or
make the build fail when they differ.

## The next contributor is an agent

Assume the next change comes from an agent that sees only the files it opened,
copies the nearest example, and takes the shortest path that compiles. A
design that depends on knowing an unwritten rule will be broken that way.
Make the right path the nearest example and the wrong path fail loudly.
