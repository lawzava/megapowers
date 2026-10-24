# Workplace comments

These synthetic examples illustrate editorial choices, not a comment template.
Keep the structure the task needs, including lists or headings when useful.

## Discussion reply

Ellis has proposed a desktop-only announcement, with mobile covered later,
and asks what else to consider. Rina supports that scope and wants to share
four recommendations:
link the migration guide, identify offline mode as experimental, decide
whether to include customer quotes because permission is unconfirmed, and
propose herself to review the wording. Neither quotes nor ownership has been
agreed.

> The desktop-only scope works for me, with mobile covered later as you
> suggested. Do we want customer quotes in this announcement? Permission is
> still unconfirmed, so we'd need to resolve that before including them.
>
> I'd link the migration guide and make clear that offline mode is
> experimental. I could review the wording if that works for you.

The opening states Rina's stance on the proposed scope and raises an exception
that needs a decision. Related details share a paragraph. All four
recommendations remain. Rina speaks for herself, and the proposed assignment
remains conditional.

## Review finding

Source inspection shows that `worker.go:73` acknowledges a message before
saving it. If the save fails, the message is lost. No test has run.

> In `worker.go:73`, the message is acknowledged before it is saved. If the
> save fails, we lose the message. Please acknowledge it only after the save
> succeeds and add a test for a failed save. I haven't run a test for this.

One paragraph connects the location, consequence, requested fix, and
verification limit. Citing `worker.go:73` already shows direct observation, so
the note states what went untested without calling the comment a "finding" or
repeating that it came from reading the code.

## Review summary

Two inline comments on a PR flag a missing migration lock in `migrate.go:41`
and an unused config field in `config.go:19`. Unit tests pass locally, but
migration integration tests were not run.

> This shouldn't merge until the migration lock in `migrate.go:41` is fixed;
> concurrent startups can run the schema change twice and corrupt the version
> table. The other inline note is minor. Unit tests pass locally, though I
> didn't run the Postgres integration suite.

Plain paragraphs state the verdict, the material risk, and the verification
boundary without `**Verdict:**` or `**Evidence:**` labels and without
re-listing every inline comment.
