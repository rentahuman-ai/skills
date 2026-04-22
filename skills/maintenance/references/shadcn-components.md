# shadcn / base-nova Component Updates

## Commands

| Task                 | Command                                                                     |
| -------------------- | --------------------------------------------------------------------------- |
| Update all shadcn    | `bun run bump:ui`                                                           |
| Update single shadcn | `bunx shadcn@latest add <component> -c packages/ui/design-system`           |
| Check shadcn changes | `bunx shadcn@latest diff -c packages/ui/design-system`                      |
| Dry-run check        | `bunx shadcn@latest add <component> --dry-run -c packages/ui/design-system` |

This project uses `base-nova` style (`@base-ui/react` primitives, not `@radix-ui`).

## Update Workflow

**Updating all components?** Run `bun run bump:ui`. This overwrites everything — merge back customizations for components in the "merge required" list below.

**Updating a specific component?** Check its classification first:

- **Safe** -> `bunx shadcn@latest add <name> -c packages/ui/design-system --overwrite`
- **Merge required** -> overwrite, then re-add customizations (see merge patterns below)
- **Never overwrite** -> skip entirely, these are project-specific

## Component Classification

Everything in `packages/ui/design-system/src/components/ui/` not listed below is safe to overwrite.

### Safe to overwrite

Standard shadcn/base-nova components with no project-specific modifications:

```
accordion.tsx       alert-dialog.tsx    alert.tsx
aspect-ratio.tsx    avatar.tsx          badge.tsx
breadcrumb.tsx      button-group.tsx    calendar.tsx
card.tsx            carousel.tsx        chart.tsx
checkbox.tsx        collapsible.tsx     context-menu.tsx
direction.tsx       dropdown-menu.tsx   field.tsx
hover-card.tsx      input-otp.tsx       item.tsx
label.tsx           menubar.tsx         native-select.tsx
navigation-menu.tsx pagination.tsx      popover.tsx
progress.tsx        radio-group.tsx     resizable.tsx
scroll-area.tsx     select.tsx          separator.tsx
sheet.tsx           skeleton.tsx        slider.tsx
sonner.tsx          switch.tsx          table.tsx
tabs.tsx            textarea.tsx        toggle-group.tsx
toggle.tsx          tooltip.tsx
```

### Merge required

Shadcn components with project-specific additions. After overwriting, re-add the customizations listed:

| Component     | Customizations                                                           |
| ------------- | ------------------------------------------------------------------------ |
| `button.tsx`  | Size variants: `xs`, `sm`, `lg`, `icon`, `icon-xs`, `icon-sm`, `icon-lg` |
| `command.tsx` | Custom keyboard handling                                                 |
| `dialog.tsx`  | Responsive modifications                                                 |
| `drawer.tsx`  | Custom animations                                                        |
| `form.tsx`    | Custom error handling                                                    |
| `input.tsx`   | Input-group integration                                                  |
| `sidebar.tsx` | Custom mobile handling                                                   |

### Never overwrite

Project-specific components not from shadcn:

```
hybrid-dialog.tsx         hybrid-alert-dialog.tsx
hybrid-hover-card.tsx     hybrid-tooltip.tsx
data-table/               code-block/
settings-card.tsx         input-group.tsx
empty.tsx                 section.tsx
grid.tsx                  frame.tsx
border-grid.tsx           background-effects.tsx
spinner.tsx               kbd.tsx
copy-button.tsx           combobox.tsx
shortcut-kbd.tsx          shortcut-tooltip.tsx
simple-form.tsx
```

## Merge Patterns

### Pattern 1: Button Variants

After overwriting `button.tsx`, re-add these custom size variants to the `cva()` call:

```tsx
size: {
  // ... shadcn defaults (default) ...
  xs: "h-6 gap-1 rounded-[min(var(--radius-md),10px)] px-2 text-xs ...",
  sm: "h-7 gap-1 rounded-[min(var(--radius-md),12px)] px-2.5 text-[0.8rem] ...",
  lg: "h-9 gap-1.5 px-2.5 ...",
  icon: "size-8",
  "icon-xs": "size-6 rounded-[min(var(--radius-md),10px)] ...",
  "icon-sm": "size-7 rounded-[min(var(--radius-md),12px)] ...",
  "icon-lg": "size-9",
},
```

Strategy: accept all shadcn changes to base styles, then re-add these variants after shadcn's defaults.

### Pattern 2: Composition Components

`hybrid-*.tsx` components wrap shadcn primitives (e.g., `hybrid-dialog.tsx` wraps `dialog.tsx` + `drawer.tsx`). When updating:

1. Overwrite the underlying primitives (`dialog.tsx`, `drawer.tsx`)
2. Check if primitive APIs changed (exports, props)
3. Update imports in the hybrid component if needed
4. Test responsive behavior (hybrid components switch between mobile/desktop)

## Coupled Packages

When updating these npm packages, re-test the associated component:

| Package                  | Component       | Notes                               |
| ------------------------ | --------------- | ----------------------------------- |
| `@base-ui/*`             | All             | Core primitives for base-nova style |
| `cmdk`                   | `command.tsx`   |                                     |
| `vaul`                   | `drawer.tsx`    | Also affects `hybrid-dialog.tsx`    |
| `embla-carousel-react`   | `carousel.tsx`  |                                     |
| `react-day-picker`       | `calendar.tsx`  |                                     |
| `recharts`               | `chart.tsx`     |                                     |
| `react-resizable-panels` | `resizable.tsx` |                                     |
| `sonner`                 | `sonner.tsx`    |                                     |
| `input-otp`              | `input-otp.tsx` |                                     |

## Verification

After any shadcn update:

1. Typecheck: `bun run typecheck --filter=@ui/design-system`
2. If errors, fix and typecheck again
3. For merge-required components, verify customizations are preserved
4. Visual spot-check in `bun run dev`
