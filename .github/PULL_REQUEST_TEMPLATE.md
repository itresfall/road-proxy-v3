## Summary

Describe what changed and why.

## Type

- [ ] Bug fix
- [ ] Feature
- [ ] Documentation
- [ ] Compatibility profile
- [ ] Refactor / maintenance

## Compatibility Profile Checklist

Fill this section if you added or changed `compat-profiles/*.json`.

- [ ] The profile ID matches the filename.
- [ ] `plugin_name` points to an existing `plugins/<name>/plugin.json`.
- [ ] ROAD scope is direct/LAN/local IP:port only.
- [ ] Steam/EOS/relay support is not claimed without direct evidence.
- [ ] Required launch arguments are documented.
- [ ] Evidence is included: game version, ROAD version, OS, transport path, player count, result.
- [ ] Logs/screenshots are anonymized.

## Validation

- [ ] `go test ./...`
- [ ] `go run ./cmd/road validate --all-configs`
- [ ] `go test ./internal/repoqa -run TestRepositoryCompatProfilesLoad -count=1` if compatibility profiles changed

## Notes

Add limitations, follow-up work, or known risks.
