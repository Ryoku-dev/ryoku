# Lithic for the web

The web library is plain CSS and ES modules. It has no package dependency, build step, runtime timer, or network requirement unless the app asks it to fetch a palette.

## Files

| File | Purpose |
|---|---|
| `lithic.css` | One-include stylesheet that imports tokens, fonts, foundation, controls, segmented, lists, and overlays. |
| `lithic.js` | One-include module that loads every component module and re-exports the core API. |
| `core.js` | Registry, initialization, mode, look, reduced-motion, role application, and palette exports. |
| `palette.js` | Palette loading, role mapping, contrast, mixing, and tone helpers. |
| `fonts.css` | Geologica and Martian Mono, loaded from `../go/fonts`, the library's one copy of the faces. |
| `foundation.css` | Base type, cut geometry, state primitives, focus, and interaction recipes. |
| `controls.css` and `controls.js` | Controls presentation and behavior, including ChordField, ColorField, and PathField. |
| `segmented.css` and `segmented.js` | Segmented presentation and behavior. |
| `lists.css` and `lists.js` | Lists, containers, state, and progress presentation and behavior, including SettingRow, the ScrollRail utility, DesignTile, DesignGallery, and ListEditor. |
| `overlays.css` and `overlays.js` | Overlays, navigation, data, and app-frame presentation and behavior. |
| `tokens.css` and `tokens.js` | Generated views of the shared token source. Do not edit them by hand. |

## Include it

Serve `library/web/` without changing the relative paths between its files. Include `lithic.css` first, then import `lithic.js` and call `Lithic.init()`:

```html
<link rel="stylesheet" href="./lithic.css">
<script type="module">
  import Lithic from "./lithic.js";
  Lithic.init();
</script>
```

`Lithic.init(root)` accepts a `Document`, `ShadowRoot`, or styled element and defaults to `document`. A document puts roles and state attributes on its `documentElement`. A shadow root puts them on its host. Initialize each independent root once.

## Palette input

Without `?palette=`, initialization applies the signature fallback from `tokens.js`. `?mode=follow`, `?mode=dark`, and `?mode=light` select the mode. Follow is the default. Forced dark or light keeps signature neutrals and takes only palette accents. `?contrast=standard|high` and `?tint=off|low|medium` select the look; standard and off are the defaults, and an unknown value is ignored.

A normal page cannot read `~/.cache/ryoku/colors.json` or `/etc/os-release`. A Ryoku app should expose a narrow daemon route that serves only the resolved palette input, then point the page at it:

```text
GET /api/theme/palette
Content-Type: application/json

{ ...the current ~/.cache/ryoku/colors.json object... }
```

```html
<a href="/settings.html?palette=/api/theme/palette">Settings</a>
```

Rashin can add a read-only `/api/theme/palette` handler to its local daemon and return the resolved palette object. The web renderer then calls `Lithic.init(document)` and needs no filesystem bridge. When a named scheme is active, the endpoint can return either `{ "themePalette": { ... } }` or `{ "roles": { ... }, "source": "scheme" }`. A raw Matugen `colors.json` object is detected as a wallpaper palette. When there is no palette, the endpoint can return `{ "roles": {}, "source": "signature", "edition": "arch" }`; this is the only way a browser receives the host edition accent because the page cannot read `/etc/os-release`.

For a palette update, fetch the endpoint again, call `rolesFromPalette(payload, {mode, contrast, tint})`, then call `Lithic.applyRoles(roles, rootElement)`. The host decides when to refresh. Lithic does not poll.

Palette role changes always snap. Core marks the root with `data-l-snap` for two animation frames whenever it writes roles, and `foundation.css` disables transitions inside that root while the marker is present. Components animate state changes, never palette changes.

## Entry module

`core.js` owns the API. `lithic.js` imports every component module and re-exports the same default and named exports, so application code normally imports `lithic.js`. The default export is `Lithic`:

- `await Lithic.init(root = document)` applies signature roles synchronously before fetching an optional palette, then applies the resolved roles, installs focus and reduced-motion tracking, and runs every registered module once for that root. It returns the final role object.
- `Lithic.setMode(mode)` accepts `follow`, `dark`, or `light`, reapplies the current palette to every initialized root without colour interpolation, and returns the selected mode.
- `Lithic.setLook({contrast, tint})` accepts `contrast` of `standard` or `high` and `tint` of `off`, `low`, or `medium`; an omitted key keeps its current value. It reapplies the current palette to every initialized root without colour interpolation and returns the look. An unknown value throws a `TypeError`. `init` reads the starting look from the `contrast` and `tint` URL parameters.
- `Lithic.setReduce(on)` accepts `true`, `false`, or `null`. True requests reduced motion; false or null clears that app request. The effective state always also honours `prefers-reduced-motion`. It returns the app request.
- `Lithic.applyRoles(roles, element = document.documentElement)` writes role custom properties and the resolved colour mode without interpolation.
- `Lithic.rolesFromPalette(palette, {mode = "follow", contrast = "standard", tint = "off"})` exposes the palette resolver.
- `Lithic.register(name, initFn)` registers a component initializer. Controls, segmented, lists, and overlays call this at module evaluation. Existing roots run a newly registered initializer immediately; future roots run it during `init`. The initializer receives `(root, Lithic)` and runs once per root.

The same functions are named exports from both `core.js` and `lithic.js`. Application code uses the aggregate `lithic.js`; component modules use `core.js`.

Keyboard navigation sets `data-l-keyboard` on the initialized root while Tab, arrow, Home, End, Page Up, or Page Down keys drive focus. Pointer or touch input clears it. Focus rings therefore appear for keyboard navigation and do not flash after pointer clicks.

## Writing a component module

Register behavior from the leaf module through `core.js`:

```js
import {register} from "./core.js";

register("controls", root => {
  // Initialize controls under root
});
```

Never import `lithic.js` from `controls.js`, `segmented.js`, `lists.js`, `overlays.js`, or another component module. The aggregate imports those modules itself. Importing the aggregate back from a leaf creates a cycle and can access the registry before its binding is initialized.

## Palette module

`palette.js` has no dependency other than the generated token object:

- `rolesFromPalette(palette, {mode = 'follow', contrast = 'standard', tint = 'off'})` returns all Lithic roles plus `dark`, `jasperInk` and `mineralFields`. It throws a `TypeError` for an unknown mode, contrast or tint. It runs the steps of [palette.md](../../palette.md#3-mapping-palette-roles-to-lithic-roles) in order:
  1. A signature input (no palette role) returns the signature table for the mode and contrast as it stands; an edition in its descriptor replaces gold and gold ink, with on-gold ink from `inkOn`.
  2. Wallpaper and scheme inputs tone every role at its step on the tone ladder for the mode and contrast, dyed from the token mappings: neutrals from `surface` and `onSurface` when the mode is not forced (forced modes keep the signature neutrals), and the accents always. The selection takes its hue from `secondaryContainer` mixed toward `primaryContainer` by `alpha.selMix`, `onSel` has its own step, and `onSel2` is `onSel` mixed toward `sel` by `alpha.onSel2`.
  3. Tint `low` or `medium` multiplies the neutral dye's chroma by `dye.tint`; it changes no lightness. The signature and the forced modes are not tinted.
  4. Jade, lapis, and amber stay fixed by mode, mineral fields are toned at the ladder's `mineralField` step, and the token scrim alpha applies.
- `CONTRASTS` (`standard`, `high`) and `TINTS` (`off`, `low`, `medium`) are the frozen look options.
- `contrastRatio(left, right)` returns the WCAG contrast ratio for two supported colour values.
- `inkOn(fill)` returns black or white ink, whichever has greater contrast on the fill.
- `mix(from, to, amount)` linearly mixes all RGBA channels. `amount` is from 0 through 1.
- `withAlpha(color, alpha)` multiplies the colour alpha by a value from 0 through 1.
- `stone(roles, tone)` returns the state mineral for `idle`, `alive`, `becoming`, `attention`, `paused`, or `failed`.
- `mineralField(roles, tone)` returns the mineral field fill for the same tones.
- `loadPalette(url)` fetches and validates a JSON object. It accepts raw `colors.json`, a `{themePalette}` response, or a `{roles, source}` descriptor.
- `applyRoles(roles, element = document.documentElement)` writes the runtime role properties.
- `ROLE_KEYS` is the frozen ordered list of canonical roles.

Colour helper inputs accept token hex values and the modern `rgb()` strings returned by these helpers. Application code should consume roles rather than raw palette keys.

## Runtime role properties

`applyRoles` writes these inheritable properties:

- Planes: `--lithic-ground`, `--lithic-stone1`, `--lithic-stone2`, `--lithic-stone3`, `--lithic-stone4`, `--lithic-slab`, `--lithic-field`, `--lithic-vein`, `--lithic-edge`
- Ink: `--lithic-fg`, `--lithic-fg2`, `--lithic-fg3`
- Selection: `--lithic-sel`, `--lithic-on-sel`, `--lithic-on-sel2`
- Action: `--lithic-gold`, `--lithic-on-gold`, `--lithic-gold-ink`
- Minerals: `--lithic-jade`, `--lithic-lapis`, `--lithic-amber`, `--lithic-jasper`, `--lithic-jasper-ink`
- Overlay: `--lithic-scrim`
- Mineral fields: `--lithic-mineral-field-alive`, `--lithic-mineral-field-becoming`, `--lithic-mineral-field-attention`, `--lithic-mineral-field-failed`

It also writes `data-l-mode="dark|light"` and the matching `color-scheme`. Core manages `data-l-palette`, `data-l-requested-mode`, `data-l-keyboard`, `data-l-reduce`, and the transient `data-l-snap`; those are managed state, not application inputs.

Generated token properties use `--lithic-<group>-<kebab-key>`. Lengths are CSS pixels, duration values are milliseconds, the motion curve is a complete `cubic-bezier()`, and font values are complete family lists. Foundation consumes the following generated properties:

- Space: `--lithic-space-s1` through `--lithic-space-s6`
- Cuts: `--lithic-cut-token`, `--lithic-cut-small`, `--lithic-cut-mid`, `--lithic-cut-large`, `--lithic-cut-notch`
- Type: `--lithic-type-<name|sentence|section|title|row|body|proof|label|caption|detail|group|mono|mono-sm>-<family|weight|size|line>`
- Fonts: `--lithic-font-sans`, `--lithic-font-mono`, `--lithic-font-icons`
- Motion: `--lithic-motion-curve`, `--lithic-motion-press`, `--lithic-motion-hover-in`, `--lithic-motion-hover-out`, `--lithic-motion-focus-in`, `--lithic-motion-focus-out`, `--lithic-motion-select`
- Alpha: `--lithic-alpha-disabled`, `--lithic-alpha-hover-fill`, `--lithic-alpha-hover-ink`, `--lithic-alpha-press-row`, `--lithic-alpha-hatch`
- Size: `--lithic-size-blade-track`, `--lithic-size-chart-hatch-gap`, `--lithic-size-chart-hatch-line`, `--lithic-size-control-press-cut-grow`, `--lithic-size-control-press-sink`, `--lithic-size-flake-height-ratio`, `--lithic-size-flake-nominal`, `--lithic-size-flake-outline`, `--lithic-size-flake-width-ratio`, `--lithic-size-focus-cut-grow`, `--lithic-size-focus-ring-gap`, `--lithic-size-focus-ring-width`, `--lithic-size-hairline`

The palette module consumes these `tokens.js` keys directly: `signature`, `tone`, `dye`, `minerals`, `palette.accents`, `palette.dye`, `edition`, `alpha.scrimDark`, `alpha.scrimLight`, `alpha.selMix`, and `alpha.onSel2`.

## Foundation classes

### Type and glyphs

`l-type-name`, `l-type-sentence`, `l-type-section`, `l-type-title`, `l-type-row`, `l-type-body`, `l-type-proof`, `l-type-label`, `l-type-caption`, `l-type-detail`, `l-type-group`, `l-type-mono`, and `l-type-mono-sm` apply the generated type records. They set the type only, never the ink, and never transform case. Write sentence case in source.

Pair `l-type-detail` with `fg2` for setting details and card secondary lines. Keep `l-type-caption` with `fg3` for timestamps, counts, hints, and placeholders only. `l-type-mono` and `l-type-mono-sm` are for literal data (addresses, paths, keys), never for counts or proofs.

`l-numeric` sets tabular figures, so counts, proofs and values that change in place keep their width. Filter counts, group counts, tablet proofs, facts and the stepper value carry it already.

The foundation also themes the browser's own surfaces from the roles: text selection is `sel` with `onSel` ink, the caret is `gold`, and scrollbars are `vein` on a clear track.

`l-glyph` renders a Material Symbols Sharp ligature. Put the symbol name in the element text, set `aria-hidden="true"` for decorative glyphs, and give an icon-only control its own accessible name.

### Cut

`l-cut` paints a clipped fill and optional inside stroke. It has no layout, padding, or text style. Set:

- `--l-fill`: fill, default `stone2`
- `--l-stroke`: inside stroke colour, default transparent
- `--l-stroke-width`: inside stroke width, default zero; use `--lithic-size-hairline` for the standard stroke
- `--l-cut`: shared 45 degree corner cut, default `cut.mid`
- `--l-cut-tl`, `--l-cut-tr`, `--l-cut-br`, `--l-cut-bl`: independent corners, each defaulting to `--l-cut`
- `--l-cut-depth`: extra cut applied to all corners, normally managed by the press recipe

The inner stroke cut is corrected by `cut - width * (2 - sqrt(2))`. Fill and stroke use crisp polygon layers in Firefox and Chromium.

### State geometry

- `l-notch` adds a top-left state notch. `--l-notch-color` defaults to `vein`; `--l-notch-size` defaults to `cut.notch`.
- `l-blade` is a track on the `field` role with a 45 degree advancing edge. `--l-fraction` is from 0 through 1. `--l-blade-fill` defaults to the current tone or `fg2`.
- `l-place-row` and `l-place-chip` mark a selection plane with the `sel` fill alone. Gliding selections (RowList, Rail, Filters, Segmented) carry the class on their slab; SectionIndex moves it with `aria-current`.
- `l-flake` is the state parallelogram. Set `data-tone` on the flake and optionally `--l-fraction` from 0 through 1. Idle is outlined; `--l-flake-background` sets its interior plane. `--l-flake-width`, `--l-flake-height`, and `--l-flake-cut` can override its token geometry.
- `l-hatch` draws 45 degree engraving. `--l-hatch-color` can override the token-alpha foreground line.
- `l-focus-ring` is a wrapper around the focusable cut element. It follows `--l-cut` and each independent corner property, then grows every non-square cut by the focus token. Set cut properties on the wrapper so they inherit into its `l-cut` child. `--l-focus-width` and `--l-focus-offset` control the ring. Use a wrapper because `l-cut` owns both pseudo-elements. The ring is two CSS pixels outside by default and appears for keyboard focus or the documented preview state.

`data-tone` accepts `idle`, `alive`, `becoming`, `attention`, `paused`, or `failed`. It exposes `--l-tone-color`, `--l-tone-ink`, and `--l-tone-field` to descendants. Idle geometry uses vein while idle words use `fg2`. Paused and attention both use amber.

```html
<span class="l-focus-ring">
  <button class="l-cut" style="--l-fill:var(--lithic-stone2)">Details</button>
</span>
```

### Recipes

Recipes are opt-in composition classes:

- `l-recipe-hover` mixes a filled `--l-hover-fill` toward white. Set `data-l-hover="transparent"` and `--l-hover-ink` for transparent controls, or `data-l-hover="plane"` and `--l-hover-plane` for a plane step. A row on `stone4` instead mixes in `fg` at `--lithic-alpha-hover-plane` because no plane sits above it.
- `l-recipe-press` deepens every cut by the token press amount and sinks content by the token distance. Set `data-l-press="row"` for the row recipe; `--l-row-fill` is its base.
- `l-recipe-focus` supplies focus transition timing to a composed focus surface. The visible ring comes from `l-focus-ring`.
- `l-recipe-disabled` applies token opacity and blocks pointer input for native `:disabled`, `aria-disabled="true"`, or `data-disabled="true"`.

For galleries and documentation renders, `data-l-state="hover|pressed|focus"` forces the matching visual state without synthetic input. Hover and pressed work on elements composed with their recipe class. Focus belongs on `l-focus-ring` or its focusable child. This is a preview hook only; application state still comes from native pointer and keyboard interaction.

Nothing animates at rest. Every transition uses the one token curve. The `data-l-reduce` state and the reduced-motion media query set all transition and animation durations to zero.

## Component reference

The tables list the app-facing markup, the attributes that CSS or JavaScript reads, events dispatched by the library, and the supported static preview hooks. `data-l-state` is only for galleries and documentation. Use native state and ARIA in an application.

### Controls

Field wrappers are part of the component contract. Wrap every `l-text-field` in `l-field-control l-cut`. Search wraps its field in `l-search l-cut`. The wrapper owns the cut fill and inside stroke so focus and error lines follow all four corners. Every field (TextField, Search, the Picker anchor, and the Stepper reading) takes the `field` fill and a hairline in `edge` at rest, `fg2` on focus, and `jasper` on error. Field labels use the detail type in `fg2`; help stays caption in `fg3`.

| Component | Classes and elements | Attributes and ARIA | Events | `data-l-state` |
|---|---|---|---|---|
| Plaque | A `button` or `a` with `l-plaque` and one of `l-gold`, `l-line`, `l-tonal`, `l-quiet`, `l-on`, `l-danger`, or `l-danger-line`. Gold is the one consequential action. Tonal is the secondary verb: a `field` fill with `fg` ink and no edge, whose hover mixes `fg` in by `alpha.hoverPlane`. Quiet has no edge at rest and `fg2` ink. Add `l-small` for the small size and `l-glyph` or `l-working` children when needed. A surrounding `l-focus-ring` draws the cut focus ring. | Native `disabled`, `aria-disabled`, and `aria-busy` block activation. Application-owned `aria-pressed` can describe a latched plaque. | No Lithic event. The element keeps its native events. | `hover`, `pressed`, and `focus`. Put `focus` on the `l-focus-ring` or its focusable child. |
| SplitPlaque | `l-split-plaque` contains a main `l-plaque` and a trailing `l-icon-plaque` marked `data-l-split-more`. | Reads and writes `aria-expanded` on the trailing control. | Bubbled `change` on `l-split-plaque`. | The two plaque children support plaque states. |
| IconPlaque | A button with `l-icon-plaque`, a decorative `l-glyph`, and an accessible name. Add `l-icon-plaque-notch` for the optional state notch and `l-small` for the small size. It accepts the same kind classes as Plaque. | Native `disabled`, `aria-disabled`, and `aria-busy`. Use `aria-label` when the glyph is the only visible content. | No Lithic event. The element keeps its native events. | `hover`, `pressed`, and `focus` through the same plaque rules. |
| TextField | `l-field` wraps `l-field-label`, `l-field-control l-cut`, an `input.l-text-field`, and optional `l-field-help`. A reveal button inside the control uses `data-l-reveal` and is normally an `l-icon-plaque`. | `data-error` on `l-field` or `aria-invalid="true"` on the input draws the error state. A reveal control reads and writes `aria-pressed` and `aria-label`. Secret inputs use native `type="password"` and an explicit `autocomplete`. | Native field events. Activating a reveal control dispatches bubbled `change` on the input. | `focus` on `l-text-field`. |
| Search | `l-search l-cut` contains a leading `l-glyph`, an `input.l-text-field`, an optional `data-l-search-hint` node, and a `data-l-search-clear` button. | Reads the input value. The library manages `data-l-has-value` and the clear button's `hidden` state. | Clearing with the button or Escape dispatches bubbled `input` and `change` on the input. Normal typing keeps native events. | `focus` on `l-text-field`. |
| Toggle | A button with `l-toggle` and `role="switch"`. | Reads and writes `aria-checked`. Native `disabled` and `aria-disabled` block toggling. | Bubbled `input` and `change` on the toggle. | `hover`, `pressed`, and `focus`. |
| Slider | `l-slider-field` contains `input.l-slider[type="range"]`, `l-slider-rail`, `l-slider-fill`, and `l-slider-handle`. The rail is a `field` track. An optional `[data-l-slider-value]` output shows the reading. | Reads native `min`, `max`, `value`, `step`, and `disabled`. If `step` is absent, the library supplies the token keyboard step. `data-l-value-text` overrides visible and accessible value text. The library writes `aria-valuetext` and `--l-value`. | Native range `input` and `change` events. | `hover`, `pressed`, and `focus` on `l-slider`. |
| Stepper | `l-stepper` contains controls marked `data-l-stepper="decrement|increment"` (normally quiet small icon plaques), an `l-stepper-reading l-cut` field box, an `l-stepper-value` in the label type with tabular figures, and optional `l-stepper-unit`. The value element can use `role="spinbutton"` and `tabindex="0"` for arrow-key input. | Reads `data-value`, `data-min`, `data-max`, and `data-step`. Writes `data-value`, `aria-valuenow`, `aria-valuemin`, and `aria-valuemax` on the value, plus `disabled` and `aria-disabled` on the buttons. | Bubbled `input` and `change` on `l-stepper`. | The plaque buttons support plaque states; `focus` on `l-stepper-value`. |
| Filters | `l-filters` contains an `l-filter-slab l-place-chip` and buttons with `l-filter`. Counts use `l-filter-count`. Chips rest on the tonal recipe; the gliding slab draws the selection. | Reads and writes `aria-pressed` and roving `tabindex`. `data-value` supplies the selected value, otherwise trimmed button text is used. The group receives `data-value`. | Bubbled `input` and `change` on `l-filters`. | `hover` and `focus` on each `l-filter`. |
| Segmented | `l-segmented l-cut` contains two to five `button.l-segment` options of equal width. The library creates the gliding `l-segmented-thumb l-cut l-place-chip`. | Defaults to `role="radiogroup"` with `role="radio"` options. Reads and writes `aria-checked` and roving `tabindex`; `data-value` supplies the value, otherwise trimmed text is used. The group receives `data-value`, `--l-segment-count`, `--l-segment-index`, and managed `data-l-selection-ready`, which turns on the glide after the first change. Arrow keys move, Home and End jump. `disabled` or `aria-disabled` on an option, or `aria-disabled` on the group, disables it. | Bubbled `input` and `change` on `l-segmented`. | `hover` on each `l-segment`; `focus` on `l-segmented`. |
| Tabs | `l-tabs[role="tablist"]` contains `l-tabs-blade` and `button.l-tab[role="tab"]` on a `vein` baseline. Each tab names a panel with `aria-controls`. | Reads and writes `aria-selected`, roving `tabindex`, and panel `hidden`. `data-value` supplies the group value, otherwise trimmed tab text is used. The tab list receives `data-value`. | Bubbled `input` and `change` on the tab list. | `hover` and `focus` on each `l-tab`. |
| Disclosure | `l-disclosure` contains a control marked `data-l-disclosure-trigger` and `l-disclosure-content`. | Reads and writes `aria-expanded` on the trigger and writes `aria-hidden` plus `inert` on the content. The library manages `data-l-open` and `--l-disclosure-height`. | Bubbled `change` on `l-disclosure`. | The trigger supports its own component states. |

Plaque:

```html
<button class="l-plaque l-gold" type="button">
  <span class="l-glyph" aria-hidden="true">save</span>
  Save
</button>
```

TextField and its required wrapper:

```html
<label class="l-field">
  <span class="l-field-label">Machine name</span>
  <span class="l-field-control l-cut">
    <input class="l-text-field" name="machine">
  </span>
  <span class="l-field-help">Use a unique fleet name.</span>
</label>
```

Toggle:

```html
<button class="l-toggle" type="button" role="switch" aria-checked="false">
  Connect automatically
</button>
```

Segmented:

```html
<div class="l-segmented l-cut" aria-label="Contrast">
  <button class="l-segment" type="button" aria-checked="true" data-value="standard">Standard</button>
  <button class="l-segment" type="button" aria-checked="false" data-value="high">High</button>
</div>
```

`segmented.js` also exports `selectSegment(group, segment, {notify = true, focus = false})`.

### Lists, containers, state, and progress

| Component | Classes and elements | Attributes and ARIA | Events | `data-l-state` |
|---|---|---|---|---|
| Tablet | `l-tablet` contains `l-tablet__mark`, `l-tablet__copy`, `l-tablet__title`, optional `l-tablet__facts`, and `l-tablet__trailing`. The trailing area contains `l-tablet__summary`, `l-tablet__claim`, `l-tablet__proof`, and optional `l-tablet__action`. Progress uses `l-tablet__progress l-blade`. A selected Tablet outside a RowList adds `l-place-row` for the `sel` fill; inside a RowList the gliding slab draws it. | Reads `role="option"`, `aria-selected`, `data-tone`, `data-value`, and `data-density="compact"`. Tablets inside a RowList must carry `role="option"`. | Selection through a RowList dispatches `lithic-select`. A standalone Tablet emits no Lithic event. | `hover` and `pressed`. |
| RowList | `l-row-list` contains `l-row-list__items`, optional `l-group` sections, and `l-tablet[role="option"]` rows. The library creates `l-row-list__slab l-place-row`, the gliding selection, and `l-row-list__thumb`. | The library sets `role="listbox"`, `tabindex`, `aria-activedescendant`, `data-l-scrollable`, and `data-l-selection-ready`. It reads child `role`, `aria-selected`, and `data-value`. Arrow keys, Home, End, Page Up, Page Down, Enter, and Space operate the list. | Bubbled `lithic-select` from the selected Tablet with `detail.index`, `detail.row`, and `detail.value`. | `focus` on `l-row-list`. |
| GroupHeader | `l-group` contains `l-group-header`, `l-group-header__title`, `l-group-header__count`, an `l-group-header__toggle`, and `l-group__rows`. | The toggle uses `aria-controls` to name the rows and reads and writes `aria-expanded`. Empty groups receive native `hidden`. | Bubbled `lithic-disclosure` from the toggle with `detail.expanded` and `detail.target`. | The toggle supports IconPlaque states. |
| Skeleton | `l-skeleton` contains one or more `l-skeleton__row` elements with `l-skeleton__bar` children. | Reads `aria-busy="true"` or `data-active="true"` and mirrors the result to managed `data-l-active`. | None. | None. |
| Empty | `l-empty` contains `l-empty__frame l-cut`, `l-empty__title`, `l-empty__body`, and optional `l-empty__actions`. | No component attribute is read. Give the frame glyph `aria-hidden="true"` and keep the title and body in normal document semantics. | None. | None. |
| Pane | `l-pane l-cut` is a list plane. Add `l-work-pane` for the work plane. | No component attribute is read. | None. | None. |
| MineralField | `l-mineral-field` can be combined with a container such as Header. | Reads `data-tone="idle|alive|becoming|attention|paused|failed"`. Tone changes animate the field unless reduced motion is active. | None. `setTone(element, tone)` updates the attribute. | None. |
| Header | `l-header` contains `l-header__identity`, `l-header__mark`, `l-header__name`, optional `l-header__proof`, `l-header__note`, `l-marks`, `l-header__acts`, and `l-header__below`. | No Header-specific attribute is read. A combined MineralField reads `data-tone`. | None. | Child controls support their own states. |
| Slab | `l-slab l-cut` is a content plane that lifts: the `slab` fill with a `vein` hairline inside the cut. Successive `l-slab-row` children are divided by a `vein` line across the inner width with `s3` above and below. Aside uses the same plane. | No component attribute is read. | None. | None. |
| Section | `l-section` contains `l-section__title`. Add `l-major` to increase the gap between consecutive major sections. | No component attribute is read. | None. | None. |
| Facts | A `dl.l-facts` contains `l-facts__row` wrappers around `dt` and `dd`. Add `l-literal` to values that need the mono type. Labels keep the 140 column, values start `s4` after it, either side wraps, and a row grows past 44 to fit its taller side. | No component attribute is read. | None. | None. |
| Aside | `aside.l-aside.l-cut` contains `l-aside__title` and optional `l-aside__preview l-cut`. | No component attribute is read. | None. | None. |
| DraftColumns | `l-draft-columns` contains `l-draft-columns__form` and an `l-aside`. It changes from a vertical flow to form and aside columns at the CSS breakpoint. | No component attribute is read. | None. | Child controls support their own states. |
| Claim | `l-claim` normally contains an `l-flake` and a state word. | Reads `data-tone` through the shared tone rules. | None. | None. |
| Marks | `l-marks` contains ordered `l-marks__mark` spans, set apart by space. Add `l-disposable` to the disposable mark. The library hides trailing marks that do not fit. | Native `hidden` on overflowed marks is managed by the library. | None. | None. |
| Sentence | `l-sentence` contains normal prose. Use `l-literal` for code or data spans; the sentence keeps one ink. | No component attribute is read. | None. | None. |
| BladeMeter | `l-blade l-blade-meter` is normally a `role="meter"` element. | Reads `aria-valuemin`, `aria-valuemax`, and `aria-valuenow`. At the token threshold it manages `data-tone="attention"`. | None. | None. |
| BladeProgress | `l-blade l-blade-progress` is normally a `role="progressbar"` element. | Reads `aria-valuemin`, `aria-valuemax`, `aria-valuenow`, `aria-busy`, `data-active`, and `data-indeterminate="true"`. It writes `--l-fraction` and managed `data-l-active`. | None. | None. |
| Working | `l-working` is normally a `role="status"` element. The library creates the token-counted `l-working__flake` children if they are absent. | Reads `aria-busy="true"` or `data-active="true"` and writes managed `data-l-active`. `setWorking(element, active)` writes `aria-busy`. | None. | None. |
| BecomingName | `l-becoming-name` contains the resource name. | Reads `aria-valuemin`, `aria-valuemax`, and `aria-valuenow`, copies its text to managed `data-l-text`, and supplies `role="progressbar"` if absent. `setBecomingName(element, fraction, {active})` also writes `aria-busy`. | None. | None. |

Tablet:

```html
<article class="l-tablet l-notch" role="option" aria-selected="false"
         data-tone="alive" data-value="ubuntu-24.04">
  <span class="l-tablet__mark"><span class="l-glyph" aria-hidden="true">computer</span></span>
  <span class="l-tablet__copy">
    <span class="l-tablet__title">ubuntu-24.04</span>
    <span class="l-tablet__facts">ally at 192.0.2.18</span>
  </span>
  <span class="l-tablet__trailing">
    <span class="l-tablet__summary"><span class="l-tablet__claim">Running</span></span>
  </span>
</article>
```

RowList:

```html
<div class="l-row-list" aria-label="Fleet machines">
  <div class="l-row-list__items">
    <article class="l-tablet" role="option" aria-selected="true" data-value="ubuntu-24.04">
      <span class="l-tablet__mark"><span class="l-glyph" aria-hidden="true">computer</span></span>
      <span class="l-tablet__copy"><span class="l-tablet__title">ubuntu-24.04</span></span>
      <span class="l-tablet__trailing"><span class="l-tablet__summary"><span class="l-tablet__claim">Running</span></span></span>
    </article>
    <article class="l-tablet" role="option" aria-selected="false" data-value="fedora-43">
      <span class="l-tablet__mark"><span class="l-glyph" aria-hidden="true">dns</span></span>
      <span class="l-tablet__copy"><span class="l-tablet__title">fedora-43</span></span>
      <span class="l-tablet__trailing"><span class="l-tablet__summary"><span class="l-tablet__claim">Stopped</span></span></span>
    </article>
  </div>
</div>
```

`lists.js` also exports `selectRow(list, row, options)`, `setWorking(element, active)`, `setBecomingName(element, fraction, options)`, `setTone(element, tone)`, and `initLists(root)`.

### Overlays, navigation, data, and window

| Component | Classes and elements | Attributes and ARIA | Events | `data-l-state` |
|---|---|---|---|---|
| Popover | An opener marked `data-l-popover="<id>"` targets `div.l-popover.l-cut#<id>`. Add `l-menu` when it contains menu content. | The library writes `aria-controls` and `aria-expanded` on the opener, adds `aria-haspopup` for menus, and manages `popover`, `hidden`, `data-l-open`, `data-l-closing`, `inert`, and placement state. `data-l-prefer="above"` requests the upper native anchor position. `data-l-static-open` embeds a noninteractive preview. | `lithic:open` and `lithic:close` on the popover with `detail.opener`. | None. Use `data-l-static-open` for an open preview. |
| Menu | `l-menu` contains `l-menu-item` controls, optional `l-glyph`, `l-menu-item-label`, `l-menu-shortcut`, and `l-menu-divider`. Add `l-danger` to a danger item. A checked item shows a trailing `check` glyph in `fg2` in the shortcut slot instead of a `sel` fill, so it carries no shortcut. | Defaults to `role="menu"` and gives untyped items `role="menuitem"`. A `role="listbox"` menu gives untyped items `role="option"`. Reads `disabled`, `aria-disabled`, `hidden`, and item `data-value`. `aria-checked="true"` on a `menuitemradio` or `menuitemcheckbox`, or `aria-selected="true"` on an option, draws the check. Activating a `menuitemcheckbox` flips its `aria-checked`; activating a `menuitemradio` checks it and clears the other radios in the menu. It sets each item to `tabindex="-1"` and moves focus with arrow keys, Home, and End. | Bubbled `lithic:select` on the menu with `detail.item` and `detail.value`, plus the Popover open and close events. | `hover`, `focus`, and `pressed` on each `l-menu-item`. |
| Picker | `button.l-picker.l-cut` is a Popover opener and a field: `field` fill, `edge` hairline, `fg2` hairline while focused or open. It contains `l-picker-label` (detail type in `fg2`), `l-picker-value`, and a trailing `l-glyph`. Its target is an `l-menu[role="listbox"]` whose items use `role="option"`; the selected option shows the menu check. | Reads `data-l-popover`, option `data-value`, and `aria-selected`. It writes `aria-haspopup="listbox"`, `aria-expanded`, picker `data-value`, selected option ARIA, and visible value text. | Bubbled `lithic:change` on the picker with `detail.value` and `detail.item`. The listbox also emits `lithic:select`. | `hover`, `pressed`, and `focus` on the picker. |
| Dialog | A control marked `data-l-dialog="<id>"` opens `dialog.l-dialog.l-cut#<id>`. Use `l-dialog-title`, `l-dialog-body`, and `l-dialog-actions`. A close control carries `data-l-dialog-close`, whose value or `value` becomes the return value. | The opener receives `aria-haspopup="dialog"`, `aria-controls`, and `aria-expanded`. The dialog uses native `open`, `showModal()`, `close()`, and `cancel`. The library manages `data-l-closing` and `inert`. `data-l-static-open` embeds a preview and disables runtime closing. | `lithic:open` with `detail.opener` and `lithic:close` with `detail.opener` and `detail.returnValue`. Native dialog events still occur. | None. Use `open data-l-static-open` for an open preview. |
| Confirm | `dialog.l-dialog.l-confirm` contains `l-confirm-field`, an input marked `data-l-confirm-input`, and an action marked `data-l-confirm-action`. | `data-l-confirm-name` is the exact required text. The library writes native `disabled` and `aria-disabled` on the action. | Bubbled `lithic:confirm-valid` on the dialog with `detail.valid` and `detail.value`, plus Dialog events. | Child controls support their own states. |
| Tooltip | An anchor marked `data-l-tooltip="<id>"` targets `l-tooltip l-cut#<id>`. | The library adds the tooltip ID to the anchor's `aria-describedby`, writes `role="tooltip"` and `popover="manual"`, and manages `hidden` plus `data-l-open`. `data-l-static-open` embeds a preview. | None. | None. Use `data-l-static-open` for an open preview. |
| Placard | `l-placard-stack` contains `l-placard l-cut`. A placard contains `l-placard-message`, optional `l-placard-action l-cut`, and `l-placard-blade`. A close or undo control uses `data-l-placard-close`. | Dispatch `lithic:placard-show` with `detail.target` as an element or ID to open it. `data-l-autoclose` enables the token lifetime. The library manages `hidden`, `data-l-open`, and `data-l-draining`. `data-l-static-open` embeds a preview. | `lithic:open` on show and `lithic:placard-dismiss` with `detail.reason` on dismissal. | `hover`, `focus`, and `pressed` on `l-placard-action`. |
| Rail | `nav.l-rail` contains `l-rail-home`, `l-rail-primary`, `l-rail-destination`, an optional foot, and labels with `l-rail-label`. The primary group stacks destinations from the start, with the New plaque after the first destination when it is shown. The foot is either one `l-rail-settings` destination or an `l-rail-foot` group whose destinations keep their order and sit anchored to the foot; the group is the web form of `footIndex`, and three destinations with Settings at the foot reproduce the default layout. `l-notch` on a destination draws the amber update notch at its top-left (the web form of `notchIndex`). Expanded, every destination tile takes the width of the widest label plus `s1` on each side, at least `railTileExpanded` (64) and at most the rail width less `s1` on each side (80); a label wider than that wraps to centred lines at word boundaries, inside a word only when one word alone is too long, and its tile grows by one caption line per extra line. Medium and compact are unchanged. The host's own mark goes inside the Home BrandMark. The library creates `l-rail-selection l-cut`, the gliding `sel` fill of the current destination, at the shared tile width. | Reads `aria-current="page"` and `data-destination`. `data-l-show-new="false"` on the rail or its AppFrame hides the New plaque (`showNew`); leaving the plaque out of the markup does the same. The library measures the labels and writes the managed `--l-rail-tile-width` on the rail and `--l-rail-extra-lines` on each destination, again once the fonts have loaded and on every resize. Clicking a destination moves `aria-current`. Managed `data-l-selection-ready` switches the selected destination from its own fill to the gliding slab. | `lithic:navigate` on the selected destination with `detail.destination`. | `hover`, `focus`, and `pressed` on home and destination elements. |
| BrandMark | `l-brand-mark` is the `sel` square with the brand cut. Left empty, JavaScript fills it with Ryoport's SVG using `l-brand-mark-octagon`, `l-brand-mark-point`, and `l-brand-mark-pointer`. A host app writes its own inline `svg` inside it instead: the library centres it, fits it inside the square inset by its cut, and sets `color` to `onSel`, so paint the mark with `currentColor`; give at most one part `l-brand-mark__accent`, which sets that part's `color` to `jasper`. The square, size, cut, Home role, hover, press, and focus stay the library's. | The generated SVG is `aria-hidden="true"` and carries the managed `data-l-brand="library"`. Use `aria-hidden="true"` for a decorative mark or give the containing Home control its accessible name. | None. | None. |
| NewPlaque | `button.l-new-plaque.l-cut` contains `l-glyph` and optional `l-new-add`, `l-new-close`, and `l-rail-label` children. It can also be a Popover opener. | CSS reads `aria-expanded` to swap add and close glyphs. Popover behavior reads `data-l-popover` and manages the normal opener ARIA. | Popover events when used as an opener. | `hover`, `focus`, and `pressed`. |
| SectionIndex | `nav.l-section-index` contains cut anchors whose `href` points to page section IDs; its `aria-label` names the column. Wide, it is a side list whose current row is a `sel` fill. Narrower, it is a Picker at the top of the column, never a chip strip: the library builds `l-section-index-picker` with an `l-picker` and an option for every anchor. | Reads and writes `aria-current` and the `l-place-row` class on the anchors, and keeps the picker value and option selection in step. Managed `data-l-hold` preserves a clicked link until scrolling resumes. `data-l-size-class="medium|compact"` on the index or its AppFrame switches to the picker. Choosing an option follows its anchor. | `lithic:section-change` on the index with `detail.target` set to the current hash; the generated picker emits the Picker events. | `hover`, `focus`, and `pressed` on links and buttons. |
| EngravedChart | `l-engraved-chart` wraps an SVG. Its parts are `l-engraved-chart-hatch`, `l-engraved-chart-field`, `l-engraved-chart-line`, `l-engraved-chart-baseline`, `l-engraved-chart-tick`, and `l-engraved-chart-label`. Use `l-engraved-chart-empty` when no series exists. | No component attribute is read. Give the figure or SVG an accessible name when it carries information. | None. | None. |
| AppFrame | `l-app-frame` contains an `l-rail` and cut panes using `l-app-frame-pane`, `l-app-frame-list`, and `l-app-frame-work`. Optional native controls use `l-app-frame-window-controls`. The rail's Home BrandMark, with any host mark inside it, is the one mark at every size, so the expanded rail and the compact bar show the same mark. | `data-l-no-list` removes the list column. `data-l-compact-bar="false"` (`compactBar`) drops the bottom rail at the compact size, so the work pane takes the full height and the host navigates. `data-l-show-new="false"` passes to its rail. `data-l-preview-size="expanded|medium|compact"` forces a gallery size. The library otherwise measures the frame and writes `data-l-size-class`. | `lithic:size-class` with `detail.sizeClass` whenever the class changes. Escape outside an open overlay dispatches `lithic:escape`. | Child navigation and controls support their own states. |

Popover:

```html
<button class="l-plaque l-quiet" type="button" data-l-popover="server-popover">
  Server details
</button>
<div class="l-popover l-cut" id="server-popover">
  <strong class="l-type-label">mika-server</strong>
</div>
```

Dialog:

```html
<button class="l-plaque l-gold" type="button" data-l-dialog="create-dialog">Create</button>
<dialog class="l-dialog l-cut" id="create-dialog" aria-labelledby="create-title">
  <h2 class="l-dialog-title" id="create-title">Create a system</h2>
  <div class="l-dialog-body">Start fedora-43 on ally.</div>
  <div class="l-dialog-actions">
    <button class="l-plaque l-quiet" type="button" data-l-dialog-close="cancel">Cancel</button>
    <button class="l-plaque l-gold" type="button" data-l-dialog-close="create">Create</button>
  </div>
</dialog>
```

AppFrame:

```html
<div class="l-app-frame">
  <nav class="l-rail" aria-label="Application">
    <a class="l-rail-home l-cut" href="/" aria-label="Home"><span class="l-brand-mark"></span></a>
    <div class="l-rail-primary">
      <a class="l-rail-destination l-cut" href="/fleet" aria-current="page"
         data-destination="fleet">Fleet</a>
    </div>
  </nav>
  <aside class="l-app-frame-pane l-app-frame-list l-cut">List</aside>
  <main class="l-app-frame-pane l-app-frame-work l-cut">Work</main>
</div>
```

`overlays.js` exports `initOverlays(root)`.

### Settings rows, galleries, and value fields

| Component | Classes and elements | Attributes and ARIA | Events | `data-l-state` |
|---|---|---|---|---|
| SettingRow | `div.l-setting-row` contains `l-setting-row__text` (an `l-setting-row__title` in the row type, an optional `l-setting-row__mark` reading "Changed", and an optional `l-setting-row__detail` in the detail type and `fg2`, `settingsDetailGap` below), an optional `l-setting-row__reset`, and `l-setting-row__control`. The reset is a small quiet IconPlaque with the `restart_alt` glyph; put the class on its `l-focus-ring` wrapper so keyboard focus draws the cut ring. Title and detail wrap and never elide. The control sits at the right, at most `settingsControlMax` wide and `s4` from the text. Rows are siblings in a Slab, `settingsRowGap` apart, at least `factsRowHeight` high; every row after the first draws a `vein` line centred in the gap above it. The hover, press, and spotlight plane bleeds `s2` sideways and `s1` up and down, cut small. | `data-l-changed` shows the mark and reserves the reset's slot; the reset shows while the row is hovered or holds focus. The library names a reset without `aria-label` "Reset <title>". `aria-disabled="true"` marks the row unavailable: the control drops to `disabledOpacity` and becomes `inert`, and the detail turns `fg3`; write the reason into `l-setting-row__detail`. `data-l-highlight` runs the search spotlight once: the hover plane fades to clear over `motion.highlight` on the chisel curve; remove the attribute after the row's `animationend`. `data-l-activatable` makes the whole row a target: the library manages `tabindex`, the row hovers with the hover-plane recipe and presses with the row recipe, and a click outside an interactive child, Enter, or Space activates it. Give an activatable row its role, such as `button` for a hand-off row. | `lithic:reset` on the row from the reset with `detail.row`. `lithic:activate` on an activatable row with `detail.row`. | `hover`, `pressed`, and `focus` on the row; `highlight` holds the first frame of the spotlight. |
| ScrollRail | `l-scroll` on any scroll container. | Nothing is read. The thumb is `scrollThumb` thick and at least `scrollThumbMin` long, `edge` at rest and `fg3` on hover or drag, with both ends cut at 45 degrees, on a clear track. Chromium draws it with the WebKit scrollbar pseudo-elements; Firefox uses `scrollbar-width: thin` and `scrollbar-color` in the same roles and draws its own square thumb. The browser shows it only while the content overflows; dragging scrolls, a press on the track pages, and the wheel stays with the pane. | Native scroll events. | `hover` on the container. |
| DesignTile | `article.l-design-tile.l-cut` is a small-cut `slab` tile with a `vein` hairline. It contains `l-design-tile__preview` with an `img` cropped to fill at `designPreviewRatio` (an empty preview draws the Hatch placeholder) and `l-design-tile__text` holding `l-design-tile__title`, an optional `l-design-tile__resource`, `l-design-tile__status` (an optional `l-working` first child), and an optional `l-design-tile__action`, a small Plaque: `l-line` by default, `l-gold` only for the one consequential action. The second line always reserves the small plaque height. The library appends `l-design-tile__ring`, the tile's FocusRing. | `aria-current="true"` marks the look in use: a `sel` plane under the text block with `onSel` ink. `aria-selected="true"` is the selection: the whole tile on `sel`. A non-gold action on either plane takes the `on` kind. `aria-disabled="true"` marks it unavailable: the status carries the reason and the preview drops to `disabledOpacity`; the tile stays selectable. `aria-busy="true"` shows the Working mark; give the mark its own `aria-busy`. `data-l-resource="light|standard|heavy"` labels the resource mark; heavy is amber. A tile outside a gallery receives `tabindex="0"`. | `lithic:activate` on a click, Enter, or Space and `lithic:action` from the action plaque, each with `detail.tile`, `detail.value`, and `detail.index`. | `hover`, `pressed`, and `focus`. |
| DesignGallery | `div.l-design-gallery` holds `l-design-tile` children in a CSS grid of `auto-fill` columns at least `catalogueTileWidthMin` wide, `s2` apart, so the focus ring never touches a neighbour. Tiles stretch up to `catalogueTileWidthMax`. Name it with `aria-label`. | The library supplies `role="listbox"` and `role="option"`, and manages `aria-selected` and roving `tabindex`. Arrow keys move the selection by one tile or one row, Home and End jump, Enter or Space activates. A click selects and activates. | `lithic:select` from the newly selected tile, plus the tile events, each with `detail.tile`, `detail.value`, and `detail.index`. | The tile states. |
| ChordField | `div.l-chord-field` contains `button.l-chord-field__control.l-cut`, a TextField box (`field` fill, `edge` hairline, `textFieldHeight` high). Give the control an `aria-label`. The library renders the chord as `kbd.l-chord-field__key.l-cut` keycaps (`field` fill, `edge` hairline, cut token, label type, `s1` by `s2` padding, `s1` apart, no plus glyphs), or `l-chord-field__placeholder` in `fg3`, or while recording `l-chord-field__prompt` "Press a shortcut" in `fg2`, and keeps an `l-chord-field__conflict` line in `jasper` under the box. | `data-value` holds the canonical chord. `data-l-placeholder` replaces "Not set". A non-empty `data-l-conflict` draws the `jasper` hairline and the error line, which the control names with `aria-describedby`. Activating the control starts recording; the library manages `data-l-recording` and `aria-pressed`. While recording, the next key with at least one modifier, or F1 to F24 or a media key alone, records; Escape cancels; Backspace or Delete with no modifier clears; Tab with no modifier and leaving the field cancel; auto-repeat is ignored. The field never talks to the compositor. | `lithic:recording` with `detail.recording` whenever recording starts or stops, so the host can hold a compositor shortcut inhibitor while it is true. `lithic:record` with `detail.chord`. `lithic:clear`. | `hover` and `focus` on the control. `data-l-recording` on the field previews recording. |
| ColorField | `div.l-color-field` is a grid of `l-color-field__swatch` (a `textFieldHeight` square, cut token, with an `edge` hairline, the colour over the Hatch so alpha shows) and the hex TextField: an `l-field` with an optional `l-field-label`, the `l-field-control l-cut` around a mono `input.l-text-field` and an `l-color-field__disclosure` small quiet IconPlaque at the trailing edge marked `data-l-popover`, and an optional `l-field-help`. The disclosure opens `div.l-popover.l-cut.l-color-field__panel`, `settingsControlMax` wide, holding `l-color-field__swatches[role="listbox"]` of `l-menu-item[role="option"]` rows, an `l-menu-divider`, and `l-color-field__body` with a Hex `l-field` and an optional `l-color-field__alpha`: an Opacity label, a Slider from 0 to 100, and a `data-l-slider-value` output. | `data-value` holds the colour. `data-l-allow-alpha` shows the opacity slider and admits alpha hex. A swatch option carries a hex `data-value` or `data-l-role="<role>"`, which reads that palette role live; the library adds its `l-color-field__chip`. Hex takes `#rgb` or `#rrggbb`, and `#rgba` or `#rrggbbaa` (CSS order) only with alpha; it is written lowercase with eight digits only below full opacity. Enter or leaving a hex field commits; an invalid value writes `data-error`, `aria-invalid`, and the help "Use a colour like #ce5e32", keeps the typed text, and emits nothing. The library writes `--l-color`, every hex field, the slider, and `aria-selected` on the matching swatch, which shows the menu check. Choosing a swatch closes the panel. | `lithic:pick` on the field with `detail.color` when a swatch, a valid hex, or the opacity slider changes the colour. The panel emits the Popover events. | `focus` on a hex field; `hover`, `pressed`, and `focus` on the disclosure and swatch rows. `data-l-static-open` on the panel previews it open. |
| ListEditor | `div.l-list-editor`, named with `aria-label`, contains `ul.l-list-editor__items` of `li.l-list-editor__item` rows, an optional `p.l-list-editor__empty` (body type, `fg2`, shown only with no items, `s2` above the add row), one host-filled `div.l-list-editor__editor` written `hidden`, and `button.l-list-editor__add.l-plaque.l-quiet` with an `add` glyph and the host's add text. A row holds `l-list-editor__text` with `l-list-editor__title` (row type) and optional `l-list-editor__detail` (detail type, `fg2`), both wrapping, then an optional `l-list-editor__trailing` control slot. The library adds the `l-list-editor__edit` (`edit`) and `l-list-editor__remove` (`delete`) small quiet IconPlaques in focus rings when the host has not written them, and, for a reorderable list, the `l-list-editor__handle` (`drag_indicator`, `fg3`) before the text. Rows stack like SettingRow: `settingsRowGap` apart, a `vein` line centred in each gap and above the add row, at least `factsRowHeight` high; the right cluster sits `s2` apart and `s4` from the text. The open editor is a `field` plane, cut small, `s3` padding, bleeding `s2` sideways, `s2` under its row (or in place of the add row), and ends in `l-list-editor__actions`: Cancel (quiet) then Done (line), small plaques, `s2` apart and `s3` below the host fields. | `data-l-reorderable` adds the handles, pointer dragging, and Alt+Up and Alt+Down on a focused row. Done reads "Done" for an edit; for a new item it reads `data-l-done-text` (default "Add"), as QML's `doneText`. `data-l-cancel-text` (default "Cancel") labels Cancel. The library names each row by its title (`aria-labelledby`), names the plaques "Edit <title>" and "Remove <title>" unless the host did, and names the editor `role="group"` "Edit <title>" or the add text. Done is disabled while the editor carries `data-l-invalid` or holds an `:invalid` control. Enter in a one-line input means Done and Escape means Cancel. One editor is open at a time; opening another cancels the first. An editor written without `hidden` is adopted as open. The library never changes the rows: it moves only the editor and the add row's `hidden`. While dragging it writes managed `data-l-dragging` and `--l-drag-y` on the row, which follows the pointer on the hover plane; the other rows stay put. | `lithic:edit` (with `detail.index`, `detail.item`, `detail.editor`) or `lithic:add` (`detail.index` -1) when the editor opens, so the host fills it; `lithic:done` (`detail.index`, `detail.item`, `detail.adding`, `detail.editor`) after it closes, so the host reads the editor and updates its rows; `lithic:cancel` (`detail.index`, `detail.item`, `detail.adding`); `lithic:remove` (`detail.index`, `detail.item`), with no Confirm; `lithic:move` (`detail.from`, `detail.to`, `detail.item`). All on the list. | `hover`, `pressed`, and `focus` on the plaques; `dragging` on a row with `--l-drag-y` previews a drag. |
| PathField | The TextField structure with `l-path-field` on the `l-field`: an `l-field-label`, the `l-field-control l-cut` around a mono `input.l-text-field` and a trailing `l-path-field__choose` small quiet IconPlaque with the `folder_open` glyph in the reveal slot, and an optional `l-field-help`. Put the class on a `l-focus-ring` wrapper around the plaque to draw the cut ring. | `data-l-kind="folder|file"` (default folder) words the default placeholder, "Choose a folder" or "Choose a file", unless the host wrote its own. The library names the plaque "Choose <label>" unless the host did. The host validates and shows an error with `data-error` and `l-field-help`, as for TextField; the library never opens a file dialog. | `lithic:choose` on the field with `detail.kind` and `detail.input` when the plaque is activated; the host opens the portal and writes the input's value. Typing and committing (Enter or leaving the field) keep the native `change` on the input. | `focus` on the input; `hover`, `pressed`, and `focus` on the plaque. |

SettingRow:

```html
<div class="l-slab l-cut">
  <div class="l-setting-row" data-l-changed>
    <div class="l-setting-row__text">
      <span class="l-setting-row__title">Poll interval</span>
      <span class="l-setting-row__mark">Changed</span>
      <span class="l-setting-row__detail">How often the fleet is checked while it is on screen.</span>
    </div>
    <span class="l-setting-row__reset l-focus-ring">
      <button class="l-icon-plaque l-quiet l-small" type="button"><span class="l-glyph" aria-hidden="true">restart_alt</span></button>
    </span>
    <div class="l-setting-row__control">
      <button class="l-toggle" type="button" role="switch" aria-checked="true" aria-label="Poll fast"></button>
    </div>
  </div>
</div>
```

ChordField:

```html
<div class="l-chord-field" data-value="Super+Shift+Return">
  <button class="l-chord-field__control l-cut" type="button" aria-label="Shortcut for Open terminal"></button>
</div>
```

A chord is its modifiers in the order Super, Ctrl, Alt, Shift, then the key, joined with `+`. Keys follow the names Ryoku's Hub records (`Combos.qtKeyName`): letters in upper case, digits, `F1` to `F24`, `Return`, `Space`, `Tab`, `Left`, `Right`, `Up`, `Down`, `BackSpace`, `Delete`, `Home`, `End`, `Prior`, `Next`, `Insert`, `Print`, `minus`, `equal`, `comma`, `period`, `slash`, `backslash`, `semicolon`, `apostrophe`, `bracketleft`, `bracketright`, `grave`, `KP_0` to `KP_9` whichever way NumLock sits, `KP_Enter`, `KP_Add`, `KP_Subtract`, `KP_Multiply`, `KP_Divide`, `KP_Decimal`, and the `XF86Audio` and `XF86MonBrightness` media keys. Letters and digits come from `KeyboardEvent.key` when it is Latin, so a remapped layout records the letter it types, and fall back to `KeyboardEvent.code` otherwise, so Shift with a digit records the digit and a non-Latin layout still records a key. Punctuation and the number pad come from `code`. Keycaps show the faces of `wm.DisplayKeys`: `Return` reads Enter, `Prior` Page Up, `KP_3` Num 3, `XF86AudioRaiseVolume` Vol +.

ColorField:

```html
<div class="l-color-field" data-value="#ce5e32" data-l-allow-alpha>
  <span class="l-color-field__swatch" aria-hidden="true"></span>
  <label class="l-field">
    <span class="l-field-label">Accent</span>
    <span class="l-field-control l-cut">
      <input class="l-text-field">
      <button class="l-icon-plaque l-quiet l-small l-color-field__disclosure" type="button"
              data-l-popover="accent-colours" aria-label="Choose accent colour"><span class="l-glyph" aria-hidden="true">expand_more</span></button>
    </span>
  </label>
  <div class="l-popover l-cut l-color-field__panel" id="accent-colours" aria-label="Accent colour">
    <div class="l-color-field__swatches" role="listbox" aria-label="Palette colours">
      <button class="l-menu-item" type="button" role="option" data-l-role="gold"><span class="l-menu-item-label">Accent</span></button>
      <button class="l-menu-item" type="button" role="option" data-l-role="jade"><span class="l-menu-item-label">Jade</span></button>
    </div>
    <hr class="l-menu-divider" role="separator">
    <div class="l-color-field__body">
      <label class="l-field"><span class="l-field-label">Hex</span><span class="l-field-control l-cut"><input class="l-text-field"></span></label>
      <label class="l-color-field__alpha">
        <span class="l-field-label">Opacity</span>
        <span class="l-slider-field"><input class="l-slider" type="range" min="0" max="100" step="1" aria-label="Opacity"><span class="l-slider-rail"><span class="l-slider-fill"></span><span class="l-slider-handle"></span></span></span>
        <output data-l-slider-value></output>
      </label>
    </div>
  </div>
</div>
```

DesignGallery:

```html
<div class="l-design-gallery" aria-label="Bar styles">
  <article class="l-design-tile l-cut" data-value="orpheus" aria-current="true" aria-selected="true">
    <span class="l-design-tile__preview"><img src="orpheus.png" alt=""></span>
    <span class="l-design-tile__text">
      <span class="l-design-tile__title">Orpheus</span>
      <span class="l-design-tile__resource" data-l-resource="standard">Standard</span>
      <span class="l-design-tile__status">In use</span>
    </span>
  </article>
  <article class="l-design-tile l-cut" data-value="aurora" aria-busy="true">
    <span class="l-design-tile__preview"></span>
    <span class="l-design-tile__text">
      <span class="l-design-tile__title">Aurora</span>
      <span class="l-design-tile__resource" data-l-resource="heavy">Heavy</span>
      <span class="l-design-tile__status"><span class="l-working" role="status" aria-label="Installing" aria-busy="true"></span>Installing</span>
      <button class="l-design-tile__action l-plaque l-line l-small" type="button">Cancel</button>
    </span>
  </article>
</div>
```

PathField:

```html
<div class="l-field l-path-field" data-l-kind="folder">
  <label class="l-field-label" for="wallpapers">Wallpaper folder</label>
  <span class="l-field-control l-cut">
    <input class="l-text-field" id="wallpapers" value="/home/ops/Pictures/Wallpapers">
    <button class="l-icon-plaque l-quiet l-small l-path-field__choose" type="button"><span class="l-glyph" aria-hidden="true">folder_open</span></button>
  </span>
</div>
```

ListEditor:

```html
<div class="l-list-editor" aria-label="Startup apps" data-l-reorderable>
  <ul class="l-list-editor__items">
    <li class="l-list-editor__item">
      <div class="l-list-editor__text">
        <span class="l-list-editor__title">Firefox</span>
        <span class="l-list-editor__detail">Opens on workspace 2</span>
      </div>
      <div class="l-list-editor__trailing">
        <button class="l-toggle" type="button" role="switch" aria-checked="true" aria-label="Start Firefox"></button>
      </div>
    </li>
  </ul>
  <p class="l-list-editor__empty">Nothing starts with the desktop yet.</p>
  <div class="l-list-editor__editor" hidden>
    <label class="l-field"><span class="l-field-label">Command</span><span class="l-field-control l-cut"><input class="l-text-field" required></span></label>
  </div>
  <button class="l-list-editor__add l-plaque l-quiet" type="button"><span class="l-glyph" aria-hidden="true">add</span>Add a startup app</button>
</div>
```

ChordField, ColorField, and PathField live in `controls.css` and `controls.js`, which also export `keyToken(event)`, `chordFrom(event)`, `displayKeys(chord)`, `renderChordField(field)`, `parseHex(text, allowAlpha)`, `formatHex(color)`, `syncColorField(field)`, and `syncPathField(field)`. SettingRow, ScrollRail, DesignTile, DesignGallery, and ListEditor live in `lists.css` and `lists.js`, which also export `syncSettingRow(row)`, `selectTile(tile, {notify = true, focus = false})`, `openListEditor(list, item)` (null for a new item), and `closeListEditor(list, {focus = true})`.

## Browser requirements

The web kit targets current Firefox and Chromium. It relies on native `<dialog>` support for modal focus and cancellation. Popovers use the Popover API when it is available, with the same `data-l-open` behavior and a positioned fallback when it is not. CSS container queries drive component-local changes such as Header type sizing. The styles and behavior also use `:has()`, `color-mix()`, `ResizeObserver`, `MutationObserver`, and the Web Animations API.

## Gallery contract

The web gallery lives in `examples/web/`. Every page:

1. links `../../library/web/lithic.css` before importing `../../library/web/lithic.js`;
2. calls `await Lithic.init(document)`;
3. accepts `?mode=follow|dark|light`, `?contrast=standard|high`, `?tint=off|low|medium`, and optional `?palette=<url>`;
4. lays out for a 1440 pixel capture and grows downward as needed;
5. labels every component by canonical name and every demonstrated state in plain words;
6. uses fleet demonstration data and no invented desktop surface;
7. has no motion at rest.

The index links `foundation.html`, `controls.html`, `lists.html`, and `overlays.html`. SettingRow, ChordField, and ColorField are on `controls.html`; DesignTile, DesignGallery, and ScrollRail on `lists.html`; the long Rail on `overlays.html`. Component modules register behavior through `Lithic.register` rather than adding page-local behavior.

To render with this box's live palette, place a temporary copy beside the pages before starting the documented server, then remove it after capture:

```sh
cp ~/.cache/ryoku/colors.json docs/lithic/examples/web/colors.json
```

Open `foundation.html?mode=follow&palette=colors.json`. `colors.json` is local runtime input and must never be committed. The signature fallback keeps the gallery self-contained when it is absent.
