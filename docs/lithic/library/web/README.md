# Lithic for the web

The web library is plain CSS and ES modules. It has no package dependency, build step, runtime timer, or network requirement unless the app asks it to fetch a palette.

## Files

| File | Purpose |
|---|---|
| `lithic.css` | One-include stylesheet that imports tokens, foundation, controls, lists, and overlays. |
| `lithic.js` | One-include module that loads every component module and re-exports the core API. |
| `core.js` | Registry, initialization, mode, reduced-motion, role application, and palette exports. |
| `palette.js` | Palette loading, role mapping, contrast, mixing, and tone helpers. |
| `foundation.css` | Base type, cut geometry, state primitives, focus, and interaction recipes. |
| `controls.css` and `controls.js` | Controls presentation and behavior. |
| `lists.css` and `lists.js` | Lists, containers, state, and progress presentation and behavior. |
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

Without `?palette=`, initialization applies the signature fallback from `tokens.js`. `?mode=follow`, `?mode=dark`, and `?mode=light` select the mode. Follow is the default. Forced dark or light keeps signature neutrals and takes only palette accents.

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

For a palette update, fetch the endpoint again, call `rolesFromPalette(payload, {mode})`, then call `Lithic.applyRoles(roles, rootElement)`. The host decides when to refresh. Lithic does not poll.

Palette role changes always snap. Core marks the root with `data-l-snap` for two animation frames whenever it writes roles, and `foundation.css` disables transitions inside that root while the marker is present. Components animate state changes, never palette changes.

## Entry module

`core.js` owns the API. `lithic.js` imports the three component modules and re-exports the same default and named exports, so application code normally imports `lithic.js`. The default export is `Lithic`:

- `await Lithic.init(root = document)` applies signature roles synchronously before fetching an optional palette, then applies the resolved roles, installs focus and reduced-motion tracking, and runs every registered module once for that root. It returns the final role object.
- `Lithic.setMode(mode)` accepts `follow`, `dark`, or `light`, reapplies the current palette to every initialized root without colour interpolation, and returns the selected mode.
- `Lithic.setReduce(on)` accepts `true`, `false`, or `null`. True requests reduced motion; false or null clears that app request. The effective state always also honours `prefers-reduced-motion`. It returns the app request.
- `Lithic.applyRoles(roles, element = document.documentElement)` writes role custom properties and the resolved colour mode without interpolation.
- `Lithic.rolesFromPalette(palette, {mode = "follow"})` exposes the palette resolver.
- `Lithic.register(name, initFn)` registers a component initializer. Controls, lists, and overlays call this at module evaluation. Existing roots run a newly registered initializer immediately; future roots run it during `init`. The initializer receives `(root, Lithic)` and runs once per root.

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

Never import `lithic.js` from `controls.js`, `lists.js`, `overlays.js`, or another component module. The aggregate imports those modules itself. Importing the aggregate back from a leaf creates a cycle and can access the registry before its binding is initialized.

## Palette module

`palette.js` has no dependency other than the generated token object:

- `rolesFromPalette(palette, {mode})` returns all Lithic roles plus `dark` and `jasperInk`. Wallpaper and scheme inputs map neutrals and accents from token mappings, keep jade, lapis, and amber fixed by mode, tone gold ink and jasper to the token contrast minimum on `stone2`, derive `onSel2`, and apply the token scrim alpha. A signature input returns the signature roles untouched. If its descriptor includes an edition, only gold, on-gold ink, and gold ink are recalculated for that accent.
- `contrastRatio(left, right)` returns the WCAG contrast ratio for two supported colour values.
- `contrastTone(ink, background, lighter)` moves HSL lightness until the token contrast minimum is reached. `lighter` is true for a dark surface and false for a light surface.
- `inkOn(fill)` returns black or white ink, whichever has greater contrast on the fill.
- `mix(from, to, amount)` linearly mixes all RGBA channels. `amount` is from 0 through 1.
- `withAlpha(color, alpha)` multiplies the colour alpha by a value from 0 through 1.
- `stone(roles, tone)` returns the state mineral for `idle`, `alive`, `becoming`, `attention`, `paused`, or `failed`.
- `field(roles, tone)` returns the mineral field fill for the same tones.
- `loadPalette(url)` fetches and validates a JSON object. It accepts raw `colors.json`, a `{themePalette}` response, or a `{roles, source}` descriptor.
- `applyRoles(roles, element = document.documentElement)` writes the runtime role properties.
- `ROLE_KEYS` is the frozen ordered list of canonical roles.

Colour helper inputs accept token hex values and the modern `rgb()` strings returned by these helpers. Application code should consume roles rather than raw palette keys.

## Runtime role properties

`applyRoles` writes these inheritable properties:

- Planes: `--lithic-ground`, `--lithic-stone1`, `--lithic-stone2`, `--lithic-stone3`, `--lithic-stone4`, `--lithic-vein`
- Ink: `--lithic-fg`, `--lithic-fg2`, `--lithic-fg3`
- Selection: `--lithic-sel`, `--lithic-on-sel`, `--lithic-on-sel2`
- Action: `--lithic-gold`, `--lithic-on-gold`, `--lithic-gold-ink`
- Minerals: `--lithic-jade`, `--lithic-lapis`, `--lithic-amber`, `--lithic-jasper`, `--lithic-jasper-ink`
- Overlay: `--lithic-scrim`

It also writes `data-l-mode="dark|light"` and the matching `color-scheme`. Core manages `data-l-palette`, `data-l-requested-mode`, `data-l-keyboard`, `data-l-reduce`, and the transient `data-l-snap`; those are managed state, not application inputs.

Generated token properties use `--lithic-<group>-<kebab-key>`. Lengths are CSS pixels, duration values are milliseconds, the motion curve is a complete `cubic-bezier()`, and font values are complete family lists. Foundation consumes the following generated properties:

- Space: `--lithic-space-s1` through `--lithic-space-s6`
- Cuts: `--lithic-cut-token`, `--lithic-cut-small`, `--lithic-cut-mid`, `--lithic-cut-large`, `--lithic-cut-notch`
- Type: `--lithic-type-<name|sentence|section|title|row|body|proof|label|caption|group|mono|mono-sm>-<family|weight|size|line>`
- Fonts: `--lithic-font-sans`, `--lithic-font-icons`
- Motion: `--lithic-motion-curve`, `--lithic-motion-press`, `--lithic-motion-hover-in`, `--lithic-motion-hover-out`, `--lithic-motion-focus-in`, `--lithic-motion-focus-out`, `--lithic-motion-select`
- Alpha: `--lithic-alpha-disabled`, `--lithic-alpha-hover-fill`, `--lithic-alpha-hover-ink`, `--lithic-alpha-press-row`, `--lithic-alpha-hatch`, `--lithic-alpha-mineral`, `--lithic-alpha-mineral-jade`
- Size: `--lithic-size-blade-track`, `--lithic-size-chart-hatch-gap`, `--lithic-size-chart-hatch-line`, `--lithic-size-control-press-cut-grow`, `--lithic-size-control-press-sink`, `--lithic-size-flake-height-ratio`, `--lithic-size-flake-nominal`, `--lithic-size-flake-outline`, `--lithic-size-flake-width-ratio`, `--lithic-size-focus-cut-grow`, `--lithic-size-focus-ring-gap`, `--lithic-size-focus-ring-width`, `--lithic-size-hairline`

The palette module consumes these `tokens.js` keys directly: `signature.dark`, `signature.light`, `minerals.dark`, `minerals.light`, `palette.dark`, `palette.light`, `palette.accents`, `palette.toned`, `edition`, `contrast.minimum`, `alpha.scrimDark`, `alpha.scrimLight`, `alpha.onSel2`, `alpha.mineral`, and `alpha.mineralJade`.

## Foundation classes

### Type and glyphs

`l-type-name`, `l-type-sentence`, `l-type-section`, `l-type-title`, `l-type-row`, `l-type-body`, `l-type-proof`, `l-type-label`, `l-type-caption`, `l-type-group`, `l-type-mono`, and `l-type-mono-sm` apply the generated type records. They never transform case. Write sentence case in source.

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
- `l-blade` is a track with a 45 degree advancing edge. `--l-fraction` is from 0 through 1. `--l-blade-fill` defaults to the current tone or `fg2`.
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

Field wrappers are part of the component contract. Wrap every `l-text-field` in `l-field-control l-cut`. Search wraps its field in `l-search l-cut`. The wrapper owns the cut fill and inside stroke so focus and error lines follow all four corners.

| Component | Classes and elements | Attributes and ARIA | Events | `data-l-state` |
|---|---|---|---|---|
| Plaque | A `button` or `a` with `l-plaque` and one of `l-gold`, `l-line`, `l-quiet`, `l-on`, `l-danger`, or `l-danger-line`. Add `l-small` for the small size and `l-glyph` or `l-working` children when needed. A surrounding `l-focus-ring` draws the cut focus ring. | Native `disabled`, `aria-disabled`, and `aria-busy` block activation. Application-owned `aria-pressed` can describe a latched plaque. | No Lithic event. The element keeps its native events. | `hover`, `pressed`, and `focus`. Put `focus` on the `l-focus-ring` or its focusable child. |
| SplitPlaque | `l-split-plaque` contains a main `l-plaque` and a trailing `l-icon-plaque` marked `data-l-split-more`. | Reads and writes `aria-expanded` on the trailing control. | Bubbled `change` on `l-split-plaque`. | The two plaque children support plaque states. |
| IconPlaque | A button with `l-icon-plaque`, a decorative `l-glyph`, and an accessible name. Add `l-icon-plaque-notch` for the optional state notch and `l-small` for the small size. It accepts the same kind classes as Plaque. | Native `disabled`, `aria-disabled`, and `aria-busy`. Use `aria-label` when the glyph is the only visible content. | No Lithic event. The element keeps its native events. | `hover`, `pressed`, and `focus` through the same plaque rules. |
| TextField | `l-field` wraps `l-field-label`, `l-field-control l-cut`, an `input.l-text-field`, and optional `l-field-help`. A reveal button inside the control uses `data-l-reveal` and is normally an `l-icon-plaque`. | `data-error` on `l-field` or `aria-invalid="true"` on the input draws the error state. A reveal control reads and writes `aria-pressed` and `aria-label`. Secret inputs use native `type="password"` and an explicit `autocomplete`. | Native field events. Activating a reveal control dispatches bubbled `change` on the input. | `focus` on `l-text-field`. |
| Search | `l-search l-cut` contains a leading `l-glyph`, an `input.l-text-field`, an optional `data-l-search-hint` node, and a `data-l-search-clear` button. | Reads the input value. The library manages `data-l-has-value` and the clear button's `hidden` state. | Clearing with the button or Escape dispatches bubbled `input` and `change` on the input. Normal typing keeps native events. | `focus` on `l-text-field`. |
| Toggle | A button with `l-toggle` and `role="switch"`. | Reads and writes `aria-checked`. Native `disabled` and `aria-disabled` block toggling. | Bubbled `input` and `change` on the toggle. | `hover` and `focus`. |
| Slider | `l-slider-field` contains `input.l-slider[type="range"]`, `l-slider-rail`, `l-slider-fill`, and `l-slider-handle`. An optional `[data-l-slider-value]` output shows the reading. | Reads native `min`, `max`, `value`, and `step`. If `step` is absent, the library supplies the token keyboard step. `data-l-value-text` overrides visible and accessible value text. The library writes `aria-valuetext` and `--l-value`. | Native range `input` and `change` events. | `focus` on `l-slider`. |
| Stepper | `l-stepper` contains controls marked `data-l-stepper="decrement|increment"`, `l-stepper-reading`, an `l-stepper-value`, and optional `l-stepper-unit`. The value element can use `role="spinbutton"` and `tabindex="0"` for arrow-key input. | Reads `data-value`, `data-min`, `data-max`, and `data-step`. Writes `data-value`, `aria-valuenow`, `aria-valuemin`, and `aria-valuemax` on the value, plus `disabled` and `aria-disabled` on the buttons. | Bubbled `input` and `change` on `l-stepper`. | The plaque buttons support plaque states. |
| Filters | `l-filters` contains an `l-filter-slab` and buttons with `l-filter`. Counts use `l-filter-count`. | Reads and writes `aria-pressed` and roving `tabindex`. `data-value` supplies the selected value, otherwise trimmed button text is used. The group receives `data-value`. | Bubbled `input` and `change` on `l-filters`. | `hover` on each `l-filter`. |
| Tabs | `l-tabs[role="tablist"]` contains `l-tabs-blade` and `button.l-tab[role="tab"]`. Each tab names a panel with `aria-controls`. | Reads and writes `aria-selected`, roving `tabindex`, and panel `hidden`. `data-value` supplies the group value, otherwise trimmed tab text is used. The tab list receives `data-value`. | Bubbled `input` and `change` on the tab list. | `hover` on each `l-tab`. |
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

### Lists, containers, state, and progress

| Component | Classes and elements | Attributes and ARIA | Events | `data-l-state` |
|---|---|---|---|---|
| Tablet | `l-tablet` contains `l-tablet__mark`, `l-tablet__copy`, `l-tablet__title`, optional `l-tablet__facts`, and `l-tablet__trailing`. The trailing area contains `l-tablet__summary`, `l-tablet__claim`, `l-tablet__proof`, and optional `l-tablet__action`. Progress uses `l-tablet__progress l-blade`. | Reads `role="option"`, `aria-selected`, `data-tone`, `data-value`, and `data-density="compact"`. Tablets inside a RowList must carry `role="option"`. | Selection through a RowList dispatches `lithic-select`. A standalone Tablet emits no Lithic event. | `hover` and `pressed`. |
| RowList | `l-row-list` contains `l-row-list__items`, optional `l-group` sections, and `l-tablet[role="option"]` rows. The library creates `l-row-list__slab` and `l-row-list__thumb`. | The library sets `role="listbox"`, `tabindex`, `aria-activedescendant`, `data-l-scrollable`, and `data-l-selection-ready`. It reads child `role`, `aria-selected`, and `data-value`. Arrow keys, Home, End, Page Up, Page Down, Enter, and Space operate the list. | Bubbled `lithic-select` from the selected Tablet with `detail.index`, `detail.row`, and `detail.value`. | `focus` on `l-row-list`. |
| GroupHeader | `l-group` contains `l-group-header`, `l-group-header__title`, `l-group-header__count`, an `l-group-header__toggle`, and `l-group__rows`. | The toggle uses `aria-controls` to name the rows and reads and writes `aria-expanded`. Empty groups receive native `hidden`. | Bubbled `lithic-disclosure` from the toggle with `detail.expanded` and `detail.target`. | The toggle supports IconPlaque states. |
| Skeleton | `l-skeleton` contains one or more `l-skeleton__row` elements with `l-skeleton__bar` children. | Reads `aria-busy="true"` or `data-active="true"` and mirrors the result to managed `data-l-active`. | None. | None. |
| Empty | `l-empty` contains `l-empty__frame l-cut`, `l-empty__title`, `l-empty__body`, and optional `l-empty__actions`. | No component attribute is read. Give the frame glyph `aria-hidden="true"` and keep the title and body in normal document semantics. | None. | None. |
| Pane | `l-pane l-cut` is a list plane. Add `l-work-pane` for the work plane. | No component attribute is read. | None. | None. |
| MineralField | `l-mineral-field` can be combined with a container such as Header. | Reads `data-tone="idle|alive|becoming|attention|paused|failed"`. Tone changes animate the field unless reduced motion is active. | None. `setTone(element, tone)` updates the attribute. | None. |
| Header | `l-header` contains `l-header__identity`, `l-header__mark`, `l-header__name`, optional `l-header__proof`, `l-header__note`, `l-marks`, `l-header__acts`, and `l-header__below`. | No Header-specific attribute is read. A combined MineralField reads `data-tone`. | None. | Child controls support their own states. |
| Slab | `l-slab l-cut` is a quiet content plane. | No component attribute is read. | None. | None. |
| Section | `l-section` contains `l-section__title`. Add `l-major` to increase the gap between consecutive major sections. | No component attribute is read. | None. | None. |
| Facts | A `dl.l-facts` contains `l-facts__row` wrappers around `dt` and `dd`. Add `l-literal` to values that need the mono type. | No component attribute is read. | None. | None. |
| Aside | `aside.l-aside.l-cut` contains `l-aside__title` and optional `l-aside__preview l-cut`. | No component attribute is read. | None. | None. |
| DraftColumns | `l-draft-columns` contains `l-draft-columns__form` and an `l-aside`. It changes from a vertical flow to form and aside columns at the CSS breakpoint. | No component attribute is read. | None. | Child controls support their own states. |
| Claim | `l-claim` normally contains an `l-flake` and a state word. | Reads `data-tone` through the shared tone rules. | None. | None. |
| Marks | `l-marks` contains ordered `l-marks__mark` spans. Add `l-disposable` to the disposable mark. The library hides trailing marks that do not fit. | Native `hidden` on overflowed marks is managed by the library. | None. | None. |
| Sentence | `l-sentence` contains normal prose. Use `l-literal` for code or data spans and `l-attention` for the attention phrase. | No component attribute is read. | None. | None. |
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
    <span class="l-tablet__facts">ally · 192.0.2.18</span>
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
| Menu | `l-menu` contains `l-menu-item` controls, optional `l-glyph`, `l-menu-item-label`, `l-menu-shortcut`, and `l-menu-divider`. Add `l-danger` to a danger item. | Defaults to `role="menu"` and gives untyped items `role="menuitem"`. A `role="listbox"` menu gives untyped items `role="option"`. Reads `disabled`, `aria-disabled`, `hidden`, and item `data-value`. It sets each item to `tabindex="-1"` and moves focus with arrow keys, Home, and End. | Bubbled `lithic:select` on the menu with `detail.item` and `detail.value`, plus the Popover open and close events. | `hover`, `focus`, and `pressed` on each `l-menu-item`. |
| Picker | `button.l-picker.l-cut` is a Popover opener. It contains `l-picker-label`, `l-picker-value`, and a trailing `l-glyph`. Its target is an `l-menu[role="listbox"]` whose items use `role="option"`. | Reads `data-l-popover`, option `data-value`, and `aria-selected`. It writes `aria-haspopup="listbox"`, `aria-expanded`, picker `data-value`, selected option ARIA, and visible value text. | Bubbled `lithic:change` on the picker with `detail.value` and `detail.item`. The listbox also emits `lithic:select`. | `hover` and `pressed` on the picker. |
| Dialog | A control marked `data-l-dialog="<id>"` opens `dialog.l-dialog.l-cut#<id>`. Use `l-dialog-title`, `l-dialog-body`, and `l-dialog-actions`. A close control carries `data-l-dialog-close`, whose value or `value` becomes the return value. | The opener receives `aria-haspopup="dialog"`, `aria-controls`, and `aria-expanded`. The dialog uses native `open`, `showModal()`, `close()`, and `cancel`. The library manages `data-l-closing` and `inert`. `data-l-static-open` embeds a preview and disables runtime closing. | `lithic:open` with `detail.opener` and `lithic:close` with `detail.opener` and `detail.returnValue`. Native dialog events still occur. | None. Use `open data-l-static-open` for an open preview. |
| Confirm | `dialog.l-dialog.l-confirm` contains `l-confirm-field`, an input marked `data-l-confirm-input`, and an action marked `data-l-confirm-action`. | `data-l-confirm-name` is the exact required text. The library writes native `disabled` and `aria-disabled` on the action. | Bubbled `lithic:confirm-valid` on the dialog with `detail.valid` and `detail.value`, plus Dialog events. | Child controls support their own states. |
| Tooltip | An anchor marked `data-l-tooltip="<id>"` targets `l-tooltip l-cut#<id>`. | The library adds the tooltip ID to the anchor's `aria-describedby`, writes `role="tooltip"` and `popover="manual"`, and manages `hidden` plus `data-l-open`. `data-l-static-open` embeds a preview. | None. | None. Use `data-l-static-open` for an open preview. |
| Placard | `l-placard-stack` contains `l-placard l-cut`. A placard contains `l-placard-message`, optional `l-placard-action l-cut`, and `l-placard-blade`. A close or undo control uses `data-l-placard-close`. | Dispatch `lithic:placard-show` with `detail.target` as an element or ID to open it. `data-l-autoclose` enables the token lifetime. The library manages `hidden`, `data-l-open`, and `data-l-draining`. `data-l-static-open` embeds a preview. | `lithic:open` on show and `lithic:placard-dismiss` with `detail.reason` on dismissal. | `hover`, `focus`, and `pressed` on `l-placard-action`. |
| Rail | `nav.l-rail` contains `l-rail-home`, `l-rail-primary`, `l-rail-destination`, optional `l-rail-settings`, and labels with `l-rail-label`. The library creates `l-rail-selection l-cut`. | Reads `aria-current="page"` and `data-destination`. Clicking a destination moves `aria-current`. Managed `data-l-selection-ready` switches the selected destination from its own fill to the gliding slab. | `lithic:navigate` on the selected destination with `detail.destination`. | `hover`, `focus`, and `pressed` on home and destination elements. |
| BrandMark | `l-brand-mark` is an empty span. JavaScript fills it with an SVG using `l-brand-mark-octagon`, `l-brand-mark-point`, and `l-brand-mark-pointer`. | The generated SVG is `aria-hidden="true"`. Use `aria-hidden="true"` for a decorative mark or give the containing Home control its accessible name. | None. | None. |
| NewPlaque | `button.l-new-plaque.l-cut` contains `l-glyph` and optional `l-new-add`, `l-new-close`, and `l-rail-label` children. It can also be a Popover opener. | CSS reads `aria-expanded` to swap add and close glyphs. Popover behavior reads `data-l-popover` and manages the normal opener ARIA. | Popover events when used as an opener. | `hover`, `focus`, and `pressed`. |
| SectionIndex | `nav.l-section-index` contains cut anchors whose `href` points to page section IDs. Buttons receive the same visual treatment. | Reads and writes `aria-current`. Managed `data-l-hold` preserves a clicked link until scrolling resumes. `data-l-size-class="medium|compact"` can preview responsive forms. | `lithic:section-change` on the index with `detail.target` set to the current hash. | `hover`, `focus`, and `pressed` on links and buttons. |
| EngravedChart | `l-engraved-chart` wraps an SVG. Its parts are `l-engraved-chart-hatch`, `l-engraved-chart-field`, `l-engraved-chart-line`, `l-engraved-chart-baseline`, `l-engraved-chart-tick`, and `l-engraved-chart-label`. Use `l-engraved-chart-empty` when no series exists. | No component attribute is read. Give the figure or SVG an accessible name when it carries information. | None. | None. |
| AppFrame | `l-app-frame` contains an `l-rail` and cut panes using `l-app-frame-pane`, `l-app-frame-list`, and `l-app-frame-work`. Optional native controls use `l-app-frame-window-controls`. | `data-l-no-list` removes the list column. `data-l-preview-size="expanded|medium|compact"` forces a gallery size. The library otherwise measures the frame and writes `data-l-size-class`. | `lithic:size-class` with `detail.sizeClass` whenever the class changes. Escape outside an open overlay dispatches `lithic:escape`. | Child navigation and controls support their own states. |

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

## Browser requirements

The web kit targets current Firefox and Chromium. It relies on native `<dialog>` support for modal focus and cancellation. Popovers use the Popover API when it is available, with the same `data-l-open` behavior and a positioned fallback when it is not. CSS container queries drive component-local changes such as Header type sizing. The styles and behavior also use `:has()`, `color-mix()`, `ResizeObserver`, `MutationObserver`, and the Web Animations API.

## Gallery contract

The web gallery lives in `examples/web/`. Every page:

1. links `../../library/web/lithic.css` before importing `../../library/web/lithic.js`;
2. calls `await Lithic.init(document)`;
3. accepts `?mode=follow|dark|light` and optional `?palette=<url>`;
4. lays out for a 1440 pixel capture and grows downward as needed;
5. labels every component by canonical name and every demonstrated state in plain words;
6. uses fleet demonstration data and no invented desktop surface;
7. has no motion at rest.

The index links `foundation.html`, `controls.html`, `lists.html`, and `overlays.html`. Component modules register behavior through `Lithic.register` rather than adding page-local behavior.

To render with this box's live palette, place a temporary copy beside the pages before starting the documented server, then remove it after capture:

```sh
cp ~/.cache/ryoku/colors.json docs/lithic/examples/web/colors.json
```

Open `foundation.html?mode=follow&palette=colors.json`. `colors.json` is local runtime input and must never be committed. The signature fallback keeps the gallery self-contained when it is absent.
