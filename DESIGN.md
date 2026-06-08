---
name: DevBox
description: Cloud development workspace product UI for Sealos.
colors:
  action-blue: "#0884DD"
  success-green: "#039855"
  warning-orange: "#DC6803"
  danger-red: "#D92D20"
  surface-white: "#FFFFFF"
  surface-muted: "#F5F5F8"
  border-subtle: "#E4E4E7"
  text-strong: "#18181B"
  text-muted: "#71717A"
typography:
  body:
    fontFamily: "Geist, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
  title:
    fontFamily: "Geist, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "18px"
    fontWeight: 500
    lineHeight: 1.55
rounded:
  sm: "6px"
  md: "8px"
  lg: "12px"
spacing:
  sm: "8px"
  md: "16px"
  lg: "24px"
components:
  button-primary:
    backgroundColor: "{colors.action-blue}"
    textColor: "{colors.surface-white}"
    rounded: "{rounded.md}"
    padding: "8px 16px"
  panel:
    backgroundColor: "{colors.surface-white}"
    textColor: "{colors.text-strong}"
    rounded: "{rounded.lg}"
---

# Design System: DevBox

## 1. Overview

**Creative North Star: "The Workspace Control Room"**

DevBox is a product surface for technical users doing repeated operational work. Screens should be dense enough to scan, restrained enough to stay calm, and explicit about resource, network, IDE, and lifecycle state.

Avoid decorative layouts and presentation-first composition in the core app. The interface should make create/edit/detail workflows feel trustworthy and fast.

**Key Characteristics:**

- Clear form structure and predictable section order.
- Neutral surfaces with one action color and semantic status colors.
- Tight product typography with stable component dimensions.
- Error and quota states that explain the next action.

## 2. Colors

The palette is restrained: neutral work surfaces, one blue action color, and conventional semantic state colors.

### Primary

- **Action Blue** (`#0884DD`): primary actions, selected states, and links.

### Neutral

- **Surface White** (`#FFFFFF`): main panels and content areas.
- **Surface Muted** (`#F5F5F8`): quiet secondary backgrounds and pending status surfaces.
- **Border Subtle** (`#E4E4E7`): dividers, panel borders, and input strokes.
- **Text Strong** (`#18181B`): primary text.
- **Text Muted** (`#71717A`): secondary labels and helper text.

### Semantic

- **Success Green** (`#039855`): running or successful states.
- **Warning Orange** (`#DC6803`): deletion or caution states.
- **Danger Red** (`#D92D20`): failed states and blocking errors.

## 3. Typography

**Display Font:** Geist with system sans fallbacks
**Body Font:** Geist with system sans fallbacks
**Label/Mono Font:** System monospace only where code or command text is shown

**Character:** concise and utilitarian. Typography should help scanning rather than create editorial drama.

### Hierarchy

- **Title** (500, 18px): section headings and key panel titles.
- **Body** (400, 14px): form labels, table cells, descriptions, and normal UI text.
- **Label** (500, 12-14px): compact controls, status labels, and metadata.

## 4. Elevation

DevBox should use borders and tonal separation before shadows. Shadows may appear on popovers, menus, and dialogs where they clarify layering, but not as decorative card styling.

## 5. Components

### Buttons

- **Shape:** rounded medium, generally 8px or less unless matching an existing component.
- **Primary:** blue filled action with concise command text.
- **Hover / Focus:** visible state change and focus ring; avoid layout shift.
- **Secondary / Ghost:** neutral controls for non-destructive or repeated actions.

### Cards / Containers

- **Corner Style:** restrained rounded panels.
- **Background:** white panels on muted page surfaces.
- **Border:** subtle zinc/neutral border.
- **Internal Padding:** use enough spacing for forms, but keep dashboards and lists scan-friendly.

### Inputs / Fields

- **Style:** familiar product controls with stable height and clear labels.
- **Focus:** visible ring or border shift.
- **Error / Disabled:** explicit text and state color, not color-only signaling.

### Navigation

Use predictable app navigation, tabs, and tables. Avoid reinventing standard product affordances for visual novelty.

## 6. Do's and Don'ts

Do:

- Use standard controls for selects, toggles, tabs, dialogs, and menus.
- Show resource, quota, inventory, and lifecycle feedback close to the action.
- Keep bilingual copy concise and operational.
- Verify text fit in compact controls.

Don't:

- Use landing-page hero patterns inside the app.
- Add decorative gradients, nested cards, or ornamental motion to operational workflows.
- Hide v2 contract details behind v1 assumptions.
- Use color as the only error or status signal.
