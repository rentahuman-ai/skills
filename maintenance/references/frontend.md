# Frontend Housekeeping

Enforce frontend conventions across all React/UI code.

**Also enforce all rules in [code-quality-checklist.md](code-quality-checklist.md)** — universal code quality rules that apply to every domain.

## Scope

- `apps/app/` (Vite + TanStack Router — primary dashboard)
- `apps/web/` (Next.js marketing site)
- `apps/admin/` (Next.js admin dashboard)
- `apps/design/` (Next.js design system preview)
- `apps/mobile/` (Expo React Native)
- `apps/desktop/` (Tauri wrapper)
- `packages/ui/*` (design-system, ai-ui, analytics, seo, next-config)
- `packages/comcom/{app-core,app-shared,sync,api-client}`

## Source of Truth

Load the `domain-frontend` skill and the `domain-design` skill before auditing.

## Audit Checklist

Rule IDs use prefix `F` (frontend): `F-P1-1`, `F-P2-3`, etc. Work through **P1 first**, then P2, then P3.

### P1 — Must Fix

| #   | Rule                                | Violation                                                | Correct                                                                  |
| --- | ----------------------------------- | -------------------------------------------------------- | ------------------------------------------------------------------------ |
| 1   | Named exports only                  | `export default function MyComponent()` (non-page)       | `export function MyComponent()` — default only for Next.js pages/layouts |
| 2   | No barrel/index imports             | `import { Button } from '@ui/design-system'`             | `import { Button } from '@ui/design-system/components/ui/button'`        |
| 3   | Path alias imports                  | `import { X } from '../../components/foo'`               | `import { X } from '@/components/foo'` or `@ui/*`, `@comcom/*`, etc.     |
| 4   | No file extensions in imports       | `import { X } from './foo.ts'`                           | `import { X } from './foo'`                                              |
| 5   | cn() for class merging              | `className={`${baseClass} ${conditionalClass}`}`         | `className={cn(baseClass, conditional && conditionalClass)}`             |
| 6   | h-dvh not h-screen                  | `className="h-screen"` on full-height elements           | `className="h-dvh"` (dynamic viewport height)                            |
| 7   | focus-visible required              | Interactive element with no focus ring                   | `focus-visible:ring-2 focus-visible:ring-ring` on all interactives       |
| 8   | No bare outline-none                | `className="outline-none"` without focus-visible ring    | `className="outline-none focus-visible:ring-2 focus-visible:ring-ring"`  |
| 9   | Icon button aria-label              | `<Button size="icon"><TrashIcon /></Button>`             | `<Button size="icon" aria-label="Delete"><TrashIcon /></Button>`         |
| 10  | No type casts                       | `as any`, `as unknown as T`, `as Type`                   | Fix the type; use type guards or generics                                |
| 11  | kebab-case component files          | `OrganizationProfile.tsx`, `userSettings.tsx`            | `organization-profile.tsx`, `user-settings.tsx`                          |
| 12  | AlertDialog for destructive actions | `<Dialog>` for delete/irreversible actions               | `<AlertDialog>` with clear confirmation                                  |
| 13  | Images need alt text                | `<img src="..." />` or `<Image src="..." />` without alt | `alt="Description"` or `alt=""` if purely decorative                     |
| 14  | Controlled inputs need onChange     | `<input value={x} />` without handler                    | `<input value={x} onChange={handler} />` or use `defaultValue`           |

### P2 — Should Fix

| #   | Rule                                  | Violation                                                            | Correct                                                                                              |
| --- | ------------------------------------- | -------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| 1   | Boolean state naming                  | `const [open, setOpen] = ...`, `const [loading, setLoading] = ...`   | `const [isOpen, setIsOpen] = ...`, `const [isLoading, setIsLoading] = ...`                           |
| 2   | Handler naming: internal vs props     | Prop named `handleSubmit`, internal handler named `onSubmit`         | Props: `on*` (`onSubmit`, `onChange`). Internal handlers: `handle*` (`handleSubmit`, `handleChange`) |
| 3   | Early returns for loading/error/empty | Happy-path content rendered inside `if (!isLoading)`                 | `if (isLoading) return <Skeleton />` at top; happy path at bottom                                    |
| 4   | nuqs for URL state                    | `useSearchParams()` with manual state sync                           | `useQueryState('filter')` from nuqs for filters, tabs, pagination                                    |
| 5   | Intl.DateTimeFormat for dates         | `date.toLocaleDateString()`, manual format strings                   | `new Intl.DateTimeFormat(locale, options).format(date)`                                              |
| 6   | Intl.NumberFormat for numbers         | `value.toFixed(2)`, manual string concat for currency                | `new Intl.NumberFormat(locale, options).format(value)`                                               |
| 7   | Component JSDoc (50+ lines)           | Feature component >50 lines with no description                      | `/** Bookmark editor with inline tagging and URL validation. */`                                     |
| 8   | Hook JSDoc with @param/@returns       | Exported hook without docs                                           | `/** @param bookmarkId - ... @returns { data, isLoading, error } */`                                 |
| 9   | Form: autocomplete + name + type      | `<input />` missing attributes                                       | `<input name="email" type="email" autoComplete="email" />`                                           |
| 10  | Form: clickable labels                | `<label>Name</label>` not connected to input                         | `<label htmlFor="name">Name</label>` or wrap the input                                               |
| 11  | Form: inline error display            | Errors only in toast or separate section                             | Inline error below field, focus first error on submit                                                |
| 12  | Form: no paste blocking               | `onPaste={e => e.preventDefault()}` or `readOnly` on editable fields | Remove paste blocking                                                                                |
| 13  | Link for navigation                   | `<a href="...">` or `onClick` with `window.location`                 | `<Link to="...">` (TanStack Router) or `<Link href="...">` (Next.js)                                 |
| 14  | No 3+ level prop drilling             | Props passed through 3+ intermediate components                      | Extract to hook, context, or composition                                                             |
| 15  | No useEffect for derived state        | `useEffect(() => { setFullName(first + last) }, [first, last])`      | `const fullName = first + last` (compute in render)                                                  |
| 16  | Virtualize large lists                | `{items.map(...)}` with 50+ items                                    | Use `@tanstack/react-virtual` or similar for 50+ items                                               |
| 17  | No layout reads in render             | `getBoundingClientRect()`, `offsetHeight` during render              | Move to `useLayoutEffect` or resize observer                                                         |
| 18  | Hook return types (3+ members)        | Exported hook returning 3+ values with no type annotation            | Annotate the return type explicitly                                                                  |
| 19  | Hydration: date/time guards           | SSR rendering `new Date()` without guard                             | Client-only render or `suppressHydrationWarning`                                                     |

### P3 — Recommended

| #   | Rule                                   | Violation                                                           | Correct                                                             |
| --- | -------------------------------------- | ------------------------------------------------------------------- | ------------------------------------------------------------------- |
| 1   | text-balance on headings               | `<h1>Long heading text</h1>`                                        | `<h1 className="text-balance">Long heading text</h1>`               |
| 2   | text-pretty on body text               | `<p>Paragraph text</p>`                                             | `<p className="text-pretty">Paragraph text</p>`                     |
| 3   | tabular-nums for data                  | Numeric table/stat without `tabular-nums`                           | `className="tabular-nums"` on data displays                         |
| 4   | Ellipsis character                     | `"Loading..."` with three dots                                      | `"Loading\u2026"` — use `…` not `...` in UI text                    |
| 5   | Curly quotes in UI copy                | `"Click here"` with straight quotes                                 | `\u201CClick here\u201D` — use `"` `"` not `"`                      |
| 6   | Loading text ends with ellipsis        | `"Saving"`, `"Processing"`                                          | `"Saving…"`, `"Processing…"`                                        |
| 7   | min-w-0 for flex truncation            | `<div className="flex"><span className="truncate">...</span></div>` | Add `min-w-0` to the flex child                                     |
| 8   | No transition-all                      | `className="transition-all"`                                        | List explicit properties: `transition-colors`, `transition-opacity` |
| 9   | Animation duration ≤200ms              | `duration-300` on interaction feedback                              | `duration-150` or `duration-200` max for interaction feedback       |
| 10  | Respect prefers-reduced-motion         | Animation without reduced-motion variant                            | Add `motion-reduce:*` or `@media (prefers-reduced-motion)`          |
| 11  | safe-area-inset on fixed elements      | `<div className="fixed bottom-0">` on mobile-capable app            | Add `pb-[env(safe-area-inset-bottom)]`                              |
| 12  | touch-action: manipulation             | Interactive surfaces without touch optimization                     | Add `touch-action-manipulation` on tap targets                      |
| 13  | overscroll-behavior: contain on modals | Modal/sheet allows page scroll underneath                           | Add `overscroll-behavior-contain`                                   |
| 14  | Component ≤150 lines target            | Component >150 lines                                                | Extract sub-components or hooks                                     |
| 15  | No gradients unless requested          | Background gradient on component                                    | Remove; use solid backgrounds                                       |
| 16  | Empty states have CTA                  | Empty list with just "No items" text                                | Add a clear next action button                                      |
| 17  | One accent color per view              | Blue and orange accents on same page                                | Limit to one accent color                                           |
| 18  | Concentric border radius               | Card `rounded-lg` > child `rounded-lg` (same radius)                | Inner radius = outer radius minus gap                               |
| 19  | z-index from fixed scale               | `z-[999]`, `z-[1000]`                                               | Use Tailwind defaults: `z-10`, `z-20`, `z-50`                       |
| 20  | LCP images set priority                | Above-fold hero image without `priority`                            | `<Image priority ... />`                                            |
| 21  | No letter-spacing changes              | `tracking-wider` on regular body text                               | Only modify for uppercase/small-caps                                |
| 22  | No animation on keyboard nav           | Focus transitions animate during tab                                | Instant focus movement during keyboard navigation                   |
| 23  | No custom shadows                      | `shadow-[0_10px_20px_...]`                                          | Use Tailwind defaults: `shadow-sm`, `shadow-md`, `shadow-lg`        |

## Discovery

```bash
find apps/app/src -name "*.tsx" -o -name "*.ts" | sort
find apps/web/src -name "*.tsx" -o -name "*.ts" | sort
find apps/admin/src -name "*.tsx" -o -name "*.ts" | sort
find apps/mobile \( -name "*.tsx" -o -name "*.ts" \) -not -path "*/node_modules/*" | sort
find apps/desktop/src \( -name "*.tsx" -o -name "*.ts" \) -not -path "*/node_modules/*" | sort
find packages/ui packages/comcom/app-core packages/comcom/app-shared packages/comcom/sync packages/comcom/api-client \( -name "*.tsx" -o -name "*.ts" \) -not -path "*/node_modules/*" | sort
```

## Audit Approach

Spawn one **Opus sub-agent per app or package**. There is no limit on the number of agents — every package gets its own agent for thorough coverage.

Each agent:

1. Loads the `domain-frontend` skill and `domain-design` skill
2. Reads all files in its assigned app or package
3. Works through this checklist **rule by rule**, P1 first, then P2, then P3
4. Also enforces all rules in [code-quality-checklist.md](code-quality-checklist.md)
5. Returns violations referencing rule IDs:

```
Package: {name} | Violations: {n}
- [F-P1-6] h-dvh not h-screen — layout.tsx:12
  Current: `className="h-screen"`
  Should be: `className="h-dvh"`
- [Q-P2-1] Boolean naming — use-sessions.ts:34
  Current: `const [loading, setLoading] = ...`
  Should be: `const [isLoading, setIsLoading] = ...`
```

Or `Package: {name} — clean` if none found.

## Cross-Cutting References

- [shadcn-components.md](shadcn-components.md) — component integrity checks
- [knip.md](knip.md) — unused deps/files in ui/comcom packages
- [npm-packages.md](npm-packages.md) — catalogue compliance
