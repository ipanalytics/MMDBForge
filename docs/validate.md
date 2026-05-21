# Validate

`mmdbforge validate` checks sampled records against a simple JSON schema.

```bash
mmdbforge validate examples/vpn.schema.json vpn.mmdb --sample 50000
```

The schema supports required dotted fields, basic JSON-like types, numeric
ranges, and simple conditional requirements.
