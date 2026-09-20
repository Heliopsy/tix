# tix

Coding standards, architecture invariants, and test conventions live in @AGENTS.md.
Read that first.

## GitHub Actions

Always pin actions to a full commit SHA, never use a tag or branch reference alone.
Include the version as a comment for readability:

```yaml
uses: actions/checkout@<full-sha>  # v6.0.3
```

This applies to all actions added or updated, including new ones introduced during fixes or features.
