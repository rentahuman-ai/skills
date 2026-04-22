# Visual Polish

Concrete CSS techniques for shadows, borders, and radius. Supplements the visual-language reference.

## Concentric Radius

Inner radius = outer radius - padding. Same radius on both creates uneven curves (the "pillow" effect).

```css
/* Incorrect — same radius on nested elements */
.outer {
  border-radius: 16px;
  padding: 8px;
}
.inner {
  border-radius: 16px;
}

/* Correct — concentric radius */
.outer {
  --padding: 8px;
  --inner-radius: 8px;
  border-radius: calc(var(--inner-radius) + var(--padding));
  padding: var(--padding);
}
.inner {
  border-radius: var(--inner-radius);
}
```

In Tailwind: parent `rounded-xl` with `p-2` → child `rounded-lg`.

## Layered Shadows

A single box-shadow looks flat. Layer multiple shadows with increasing blur and decreasing opacity.

```css
/* Incorrect — single flat shadow */
.card {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
}

/* Correct — layered shadows */
.card {
  box-shadow:
    0 1px 2px rgba(0, 0, 0, 0.06),
    0 4px 8px rgba(0, 0, 0, 0.04),
    0 12px 24px rgba(0, 0, 0, 0.03);
}
```

All shadow offsets MUST share the same direction (single light source — typically top-down).

NEVER use pure black (`rgba(0,0,0,...)`) for shadows — use deep neutrals:

```css
/* Incorrect */
box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
/* Correct */
box-shadow: 0 4px 12px rgba(17, 24, 39, 0.08);
```

## Semi-Transparent Borders

Hardcoded border colors break across light/dark mode. Semi-transparent borders adapt automatically.

```css
/* Incorrect */
border: 1px solid #e5e5e5;
/* Correct */
border: 1px solid var(--gray-a4);
```

In Tailwind: `border-black/10` (light) or `border-white/10` (dark).

## Button Shadow Anatomy

For polished, elevated buttons, layer six techniques:

1. **Outer cut shadow** — 0.5px dark box-shadow to "cut" into the surface
2. **Inner ambient highlight** — 1px inset box-shadow on all sides
3. **Inner top highlight** — 1px inset top highlight (light from above)
4. **Layered depth shadows** — 3+ external shadows
5. **Text drop-shadow** — subtle drop-shadow on text for contrast
6. **Subtle gradient** — if you can tell there's a gradient, it's too much

```css
.button {
  background: linear-gradient(
    to bottom,
    color-mix(in srgb, var(--gray-12) 100%, white 4%),
    var(--gray-12)
  );
  color: var(--gray-1);
  box-shadow:
    0 0 0 0.5px rgba(0, 0, 0, 0.3),
    inset 0 0 0 1px rgba(255, 255, 255, 0.04),
    inset 0 1px 0 rgba(255, 255, 255, 0.07),
    0 1px 2px rgba(0, 0, 0, 0.1),
    0 2px 4px rgba(0, 0, 0, 0.06),
    0 4px 8px rgba(0, 0, 0, 0.03);
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.15);
}
```

Use sparingly — this level of polish is for primary CTAs, not every button.

## Hit Target Expansion

Expand hit targets with pseudo-elements when the visible element must stay small:

```css
.link {
  position: relative;
}
.link::before {
  content: '';
  position: absolute;
  inset: -8px -12px;
}
```
