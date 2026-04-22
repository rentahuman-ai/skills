# Visual Language

The concrete aesthetic system. How the philosophy manifests in pixels.

## Color

### Foundation: monochrome first

Every interface starts in grayscale. Color is added last, only where it carries meaning.

- **Background layers**: Use 2-3 background tones to create depth (e.g., `bg-background`, `bg-muted`, `bg-card`). NEVER more than 3 layers.
- **Text hierarchy**: Maximum 3 text colors — primary, secondary (muted), and tertiary (disabled). Derive from the same hue family.
- **Accent**: One accent color per product, used sparingly — primary buttons, active states, links. NEVER decorative.
- **Semantic colors**: Reserve red for errors/destructive, amber for warnings, green for success. NEVER use semantic colors for branding or decoration.

### How Linear uses color

- The UI is almost entirely grayscale — white, off-white, and shades of gray
- Purple accent appears only for the active item, primary CTA, and brand moments
- Issue priority, status, and labels use muted, desaturated colors — never saturated primaries
- Dark mode is the default because it reduces visual noise and lets content breathe

### Color rules

- MUST be able to remove all accent color and still have a usable, readable interface
- MUST ensure 4.5:1 contrast ratio for all text (WCAG AA)
- NEVER use color as the only way to convey information — pair with icons, text, or position
- NEVER use more than one saturated color on a single surface
- SHOULD default to dark mode in developer tools — it reduces eye strain and visual hierarchy is clearer

## Borders and dividers

Borders are a crutch. They indicate the layout isn't doing its job.

- SHOULD use spacing and background color changes to separate regions instead of borders
- When borders are necessary, MUST use subtle, low-contrast strokes (`border-border` at most)
- NEVER use borders thicker than 1px for UI structure
- MUST NOT combine a border AND a background color change at the same boundary — pick one
- SHOULD use `divide-y` for list items rather than individual borders per item

### How Vercel handles separation

- Cards have no visible borders — they float on a slightly different background
- Lists use hairline dividers (`0.5px` or `border-opacity-50`)
- Sections are separated by generous whitespace, not lines
- The only prominent border in the entire UI is the focus ring

## Shadows

Shadows communicate elevation. Use them structurally, never decoratively.

- MUST limit to 2 shadow levels: subtle (cards, dropdowns) and elevated (modals, popovers)
- NEVER use colored shadows or glows
- NEVER use `shadow-lg` or larger for inline UI elements — reserve large shadows for overlays
- SHOULD use Tailwind's default shadow scale without modification
- MUST NOT combine shadows with borders on the same element
- SHOULD layer multiple box-shadows for realistic depth rather than a single heavy shadow
- MUST keep all shadow offsets in the same direction (consistent light source)
- NEVER use pure black for shadows — use neutral colors at low opacity

## Icons

Icons are labels, not decoration. An icon without a purpose is noise.

- MUST use icons to clarify meaning, not to fill space
- MUST pair icons with text labels in navigation — icon-only nav requires memorization
- MUST use a single icon family consistently (Lucide in this project)
- NEVER use icons larger than 20px in dense UI — 16px is the default for inline icons
- NEVER use filled icons when outline variants exist (filled icons are visually heavier)
- SHOULD use icons for status indicators (checkmark, warning, error) rather than colored dots alone

### Icon sizing

| Context           | Size                           | Example                             |
| ----------------- | ------------------------------ | ----------------------------------- |
| Inline with text  | 16px (`size-4`)                | Menu items, list items, breadcrumbs |
| Button with label | 16px (`size-4`)                | Action buttons, form controls       |
| Icon-only button  | 16-20px (`size-4` to `size-5`) | Toolbar actions, close buttons      |
| Empty state       | 24-32px (`size-6` to `size-8`) | Illustrations in empty views        |
| Page header       | 20-24px (`size-5` to `size-6`) | Section icons, feature icons        |

## Spacing system

Consistent spacing creates visual rhythm. Rhythm creates trust.

- MUST use the Tailwind 4px grid (`gap-1` = 4px, `gap-2` = 8px, `gap-4` = 16px, etc.)
- MUST use the same spacing value for related elements — don't mix `gap-3` and `gap-4` in the same component
- MUST increase spacing as elements become less related (tight within a group, loose between groups)
- SHOULD use these spacing tiers:

| Relationship                       | Spacing | Tailwind             |
| ---------------------------------- | ------- | -------------------- |
| Tightly coupled (icon + label)     | 4-8px   | `gap-1` to `gap-2`   |
| Related (form field + helper text) | 8-12px  | `gap-2` to `gap-3`   |
| Grouped (card content sections)    | 16-20px | `gap-4` to `gap-5`   |
| Separated (page sections)          | 32-48px | `gap-8` to `gap-12`  |
| Major sections                     | 64-80px | `gap-16` to `gap-20` |

## Dark mode

Dark mode is not an inversion — it is a redesign.

- MUST NOT simply swap black for white — dark backgrounds need different shadow and border treatments
- MUST reduce shadow intensity in dark mode (shadows are barely visible on dark backgrounds — use subtle border/glow instead)
- MUST ensure images and illustrations work in both modes — avoid hard white backgrounds in SVGs
- SHOULD use slightly warm dark tones (`slate`, `zinc`) rather than pure black (`#000`)
- SHOULD increase spacing slightly in dark mode — dark interfaces feel denser than light ones at the same spacing

## Radius

Consistency in border radius creates visual cohesion.

- MUST use a single radius value for all interactive elements (buttons, inputs, cards, chips) — `rounded-lg` (8px) is the default
- MUST use `rounded-full` only for avatars and circular icon buttons
- NEVER mix radius values within the same component
- Nested elements MUST use concentric radius: inner = outer - gap (e.g., parent `rounded-xl` at `p-2`, child `rounded-lg`). Calculate, don't guess.
