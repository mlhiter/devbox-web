# Devbox V2 Frontend Design Notes

## Register

Product UI. The interface should feel like an operational workspace for repeated
developer tasks, not a marketing site.

## Visual Direction

- Keep the current light, utilitarian Sealos console style.
- Favor dense but readable forms, tables, drawers, dialogs, and status panels.
- Use concise labels and clear status copy. Avoid explanatory prose inside the
  app unless it prevents an operational mistake.
- Keep controls predictable: drawers for configuration, dialogs for confirmation,
  tabs for detail sections, and toast feedback for request outcomes.

## Interaction Principles

- Creation and edit flows should keep form state visible until the user commits.
- Drawer validation can update local form state, but persistence happens through
  the parent form submit action.
- Destructive and lifecycle actions need explicit confirmation or clear feedback.
- Long-running release, deploy, and retag operations should surface actionable
  error messages instead of raw infrastructure strings when possible.

## Responsive Guidance

- Prioritize form legibility and action reachability on narrow screens.
- Avoid nested cards and decorative containers around already framed tool
  surfaces.
- Keep button text short enough for Chinese and English labels.
