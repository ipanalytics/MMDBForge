# CI

Example GitHub Actions step:

```yaml
- name: Build mmdbforge
  run: go install ./cmd/mmdbforge

- name: Guard MMDB release
  run: |
    mmdbforge audit release \
      --old artifacts/vpn-previous.mmdb \
      --new artifacts/vpn-current.mmdb \
      --schema examples/vpn.schema.json \
      --policy examples/release.policy.json \
      --smoke fixtures/vpn-smoke.json \
      --sample 100000 \
      --markdown report.md \
      --html report.html
```
