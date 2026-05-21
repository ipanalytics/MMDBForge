# Diff

`mmdbforge diff` samples records from the old database, looks up the same IPs in
the new database, and reports changed records plus per-field transitions.

```bash
mmdbforge diff old.mmdb new.mmdb --sample 100000
mmdbforge diff old.mmdb new.mmdb --ips test-ips.txt --fields privacy.is_vpn,confidence
```

CI guardrails:

```bash
mmdbforge diff old.mmdb new.mmdb \
  --sample 100000 \
  --fail-threshold changed_percent=25 \
  --fail-on-missing-field confidence
```

Use `--markdown` to print a Markdown report or `--markdown report.md` to write it.
