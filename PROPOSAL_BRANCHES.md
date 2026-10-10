# Proposal Branches

Proposal branches are a historical idea, not the current Contuts workflow.

The project no longer treats multiple AI proposals and a Human build as the
primary product. The current product starts from the code that exists, chooses a
Git base commit, and builds a debug experiment around the current state.

The useful surviving principle is context preservation:

```text
source revision
  -> changed locations
    -> runtime experiment
      -> saved result
```

If proposal branches return later, they should use the same experiment model.
Each proposal would need its own Git context, target configuration, artifacts,
and rerunnable evidence. Proposal composition should not be designed before the
single-project experiment loop is useful.
