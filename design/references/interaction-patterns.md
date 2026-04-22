# Interaction Patterns

How the design philosophy manifests in behavior. The feel of the product.

## Speed as a feature

The most important design decision is performance. A fast interface with average visuals will always feel better than a slow interface with perfect visuals.

- MUST use optimistic updates for user-initiated mutations — show the result immediately, reconcile in the background
- MUST preload data for likely next actions (hover on a link → prefetch that page)
- MUST render skeleton layouts instantly — never show a blank page while loading
- NEVER show a spinner for operations under 200ms
- NEVER block the UI while waiting for a server response unless the operation is destructive
- SHOULD use streaming/incremental rendering for large datasets
- SHOULD use cursor trajectory prediction for prefetching where hover-based prefetch adds noticeable latency

### How Linear feels fast

- Every action is optimistic — creating an issue, changing status, moving between views
- Navigation is instant because views are cached and prefetched
- Keyboard shortcuts bypass all the overhead of mouse navigation
- The UI never shows loading states for common operations

## Keyboard-first

The keyboard is faster than the mouse. Design for keyboard power users, then add mouse support.

- MUST support keyboard navigation for all primary workflows
- MUST implement a command palette (`Cmd+K`) as the universal entry point
- MUST show keyboard shortcuts in tooltips and menu items
- MUST support `Escape` to dismiss any overlay, go back, or cancel
- SHOULD support single-key shortcuts for frequent actions (not just Cmd+key combos)
- SHOULD support `j/k` or arrow key navigation in lists and tables

### Command palette principles

- MUST search across all entity types (pages, actions, settings, recent items)
- MUST rank results by recency and frequency, not just text match
- MUST support nested commands ("Change status → In Progress")
- SHOULD show 5-7 results maximum — more indicates poor ranking
- SHOULD execute the top result on Enter without requiring selection

## Progressive disclosure

Show what matters now. Reveal more on demand.

- MUST present the most common action as the default, not a list of equal options
- MUST use hover/focus to reveal secondary actions — they exist but don't compete for attention
- MUST use expandable sections for advanced settings rather than showing everything at once
- NEVER show an advanced feature if the user hasn't used the basic version yet
- SHOULD use "more" menus (`...`) for tertiary actions — max 3 visible actions per row

### Disclosure hierarchy

| Level     | Visibility                           | Example                           |
| --------- | ------------------------------------ | --------------------------------- |
| Primary   | Always visible                       | Main CTA button, page title       |
| Secondary | Visible on hover/focus               | Row actions, edit buttons         |
| Tertiary  | Behind a menu or click               | Delete, export, advanced settings |
| Expert    | Behind settings or keyboard shortcut | Bulk operations, custom filters   |

## Feedback and state

Every action needs acknowledgment. The user should never wonder "did that work?"

- MUST show immediate visual feedback for every interaction (button press, toggle, selection)
- MUST use inline feedback over toasts — show the result where the action happened
- MUST show error states inline, next to the element that failed
- NEVER use success toasts for expected outcomes (saving, creating, updating)
- SHOULD use subtle state transitions (opacity, color shift) rather than dramatic animations
- SHOULD persist undo capability for destructive actions rather than asking for confirmation upfront
- SHOULD show progress indicators for multi-step flows to drive completion
- SHOULD end user flows with clear success states — the final moment disproportionately shapes perception

### State communication

| State           | Treatment                                                          |
| --------------- | ------------------------------------------------------------------ |
| Loading         | Skeleton placeholder matching the expected layout                  |
| Empty           | Illustration + one clear action to create the first item           |
| Error           | Inline message with the reason and a recovery action               |
| Success         | Silent (the result is the feedback) — or brief inline confirmation |
| Partial failure | Show what succeeded + what failed with retry                       |

## Navigation

Navigation should be invisible — the user should always know where they are without thinking about it.

- MUST use URL state for everything — filters, tabs, modal states, pagination
- MUST support browser back/forward for all navigation
- MUST highlight the current location in the sidebar/nav
- NEVER use modals for content that deserves its own URL
- SHOULD use breadcrumbs for hierarchical navigation deeper than 2 levels
- SHOULD preserve scroll position when returning to a previously visited view

### Navigation patterns by depth

| Depth                             | Pattern                             | Example                     |
| --------------------------------- | ----------------------------------- | --------------------------- |
| Top-level (3-5 items)             | Sidebar or top nav                  | Dashboard, Issues, Settings |
| Second-level (scoped to parent)   | Tabs or sub-nav within the page     | Active, Backlog, Done       |
| Third-level                       | Breadcrumbs + detail view           | Team → Project → Issue      |
| Transient (doesn't deserve a URL) | Sheet, popover, or inline expansion | Quick edit, preview         |

## Selection and multi-select

- MUST support click for single selection, Cmd+click for multi-select, Shift+click for range
- MUST show a persistent action bar when items are selected ("3 selected — Archive, Delete")
- MUST support Select All with a clear indication of scope
- SHOULD support drag-to-select for spatial layouts (kanban boards, file grids)
- SHOULD preserve selection across pagination/filtering when possible

## Drag and drop

Use sparingly. Drag and drop is powerful but invisible — the user must discover it.

- MUST provide a non-drag alternative for every drag action (menu item, keyboard shortcut)
- MUST show a clear drop target indicator during drag
- MUST support Escape to cancel a drag operation
- NEVER use drag as the only way to reorder or organize
- SHOULD use a drag handle icon to indicate draggable items (don't make the entire row draggable without visual affordance)
