# CLI Housekeeping

Enforce conventions across the CLI tool (`tcc`).

## Scope

- `apps/cli/src/**`
- `apps/cli/package.json` (bin registration)

## Source of Truth

CLAUDE.md conventions + internal CLI patterns (Commander.js, `@comcom/api-client`).

## What to Check

| Check                | Details                                                                                                |
| -------------------- | ------------------------------------------------------------------------------------------------------ |
| Command registration | Every command file in `src/commands/` is registered in `src/main.ts`                                   |
| Output consistency   | All commands use `printJson()`, `printTable()`, `printDetail()`, `printError()` — no raw `console.log` |
| Option naming        | Consistent kebab-case for CLI flags, camelCase for internal variables                                  |
| Error handling       | All API calls wrapped with proper error handling, user-friendly messages via `printError()`            |
| API client alignment | Using `@comcom/api-client` generated types, not hand-rolled API calls                                  |
| Global options       | `--api-key`, `--api-url`, `--json` available on all commands that need them                            |
| No type casting      | No `as`, `as any`, `as unknown as`                                                                     |
| Import style         | Path aliases, direct file imports                                                                      |
| Dead commands        | No command files that aren't registered or reachable                                                   |
| Help text            | Every command and option has a description                                                             |

## Discovery

```bash
find apps/cli/src -name "*.ts" | sort
```

## Audit Approach

Single Opus sub-agent (small codebase):

1. Read `src/main.ts` to get command registration list
2. Read all files in `src/commands/`
3. Cross-reference: every command file is registered, every registration points to an existing file
4. Scan each command for output pattern violations, error handling gaps, option naming inconsistencies
5. Return structured violations

## Cross-Cutting References

- [npm-packages.md](npm-packages.md) — catalogue compliance
- [knip.md](knip.md) — unused exports/deps
