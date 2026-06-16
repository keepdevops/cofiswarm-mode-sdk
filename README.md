# cofiswarm-mode-sdk

Cofiswarm component: `mode-sdk`.

- Layout: [REPO-STANDARD-LAYOUT](https://github.com/keepdevops/cofiswarmdev/blob/main/docs/REPO-STANDARD-LAYOUT.md)
- Migration: [MIGRATION-SPRINTS](https://github.com/keepdevops/cofiswarmdev/blob/main/docs/MIGRATION-SPRINTS.md)

## FHS paths

| Path | Purpose |
|------|---------|
| `/etc/cofiswarm/mode-sdk/` | config |
| `/var/lib/cofiswarm/mode-sdk/` | state |
| `/var/log/cofiswarm/mode-sdk/` | logs |

## Test

```bash
./test/scripts/assert-layout.sh mode-sdk
```
