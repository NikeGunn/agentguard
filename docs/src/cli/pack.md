# agentguard pack

Manage rule packs.

```
agentguard pack list
agentguard pack show   <name>
agentguard pack verify <name>
```

`<name>` can be a built-in (`default`, `strict`) or a user pack
(`user/my-pack` — resolved from `~/.agentguard/packs/my-pack.yaml`).

See [Rule packs](../rule-packs.md) for the YAML format.
