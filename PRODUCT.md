# Product

## Register

product

## Users

DevBox is used by developers, platform operators, and Sealos workspace administrators. Developers create and manage cloud workspaces, connect local IDEs, expose ports, mount config files or network storage, and release workspace images. Operators maintain the Kubernetes-side runtime, gateway, quota, and deployment surfaces that make those workspaces reliable.

## Product Purpose

DevBox provides cloud development workspaces on Kubernetes for Sealos. The frontend should make workspace creation, configuration, connectivity, and release workflows predictable while hiding cluster complexity. The controller and gateway services reconcile the actual DevBox runtime state.

Success means a user can create or edit a DevBox, understand resource and quota constraints, connect through the expected IDE path, and recover from lifecycle or configuration changes without needing to know Kubernetes internals.

## Brand Personality

Quiet, capable, operational. The product should feel like a dependable tool for repeated technical work rather than a marketing surface.

## Anti-references

- Marketing-style hero pages for core workflows.
- Decorative card grids, oversized claims, or visual effects that slow operational scanning.
- Surprising custom controls where standard product UI patterns are clearer.
- v1/v2 copy-paste migrations that erase v2 backend or CRD contracts.

## Design Principles

- Keep workflows close to the real cluster contract.
- Preserve v2 authority when v1 and v2 differ systemically.
- Make state and limits visible before the user submits.
- Prefer familiar controls for repeated operations.
- Split changes by product behavior so fixes stay traceable.

## Accessibility & Inclusion

Use semantic controls, visible focus states, keyboard-friendly dialogs and menus, and clear bilingual copy. Error messages should map backend or quota failures to user-actionable text without hiding important operational detail.
