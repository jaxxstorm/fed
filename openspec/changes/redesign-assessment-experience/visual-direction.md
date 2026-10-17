# Visual direction: an assessment workspace

Use an original calm light interface: warm-gray canvas, white panels, deep navy
text, restrained indigo interaction color, and semantic green/red/amber accents.
Show status with labels and icons as well as color. Use a compact persistent
sidebar and a generous reading area instead of a wall of links.

## Overview wireframe

```text
┌─────────────────┬────────────────────────────────────────────────────┐
│ Service switcher│ Ignis / Overview                 Class B · Updated │
│                 ├────────────────────────────────────────────────────┤
│ Overview        │ Your assessment, at a glance       Connect agent   │
│ Requirements    │                                                    │
│ Evidence        │ Passing requirements   Evidence gathered  Reviews  │
│ Activity        │ 24 / 115               63 / 115           39       │
│ Sources         │ [labeled segmented progress]                       │
│                 │                                                    │
│ Settings        │ What needs attention       Latest agent results    │
│ Exports         │ ! Cluster read denied      Added deployment file   │
│                 │ → Review source access     Updated 2 requirements  │
│                 │                                                    │
│                 │ Requirement categories, with counts and filters    │
└─────────────────┴────────────────────────────────────────────────────┘
```

The numbers above are illustrative fixtures, never fallback production values.
Charts are subordinate to the numbers and actions. Use a segmented horizontal
bar, not a reproduction of the supplied dashboard layouts.

## Requirement wireframe

```text
Overview / Requirements / Automating configuration management
Needs evidence                         Last updated …
Add dated drift-review results to complete this investigation.

[Next steps]          [What the agent found]
2 missing items       Deployment automation is configured.
Copy investigation    Evidence added … · Review pending

Evidence | Findings | History
Title / source / observation date / freshness / linked assertions
Select evidence → side panel with source and exact citation preview

Review supporting evidence          Record review
```

## Evidence and errors

Preview panels keep the selected rule/library context visible. Long identifiers
belong behind a Copy ID control; line references and JSON paths remain easy to
locate. An error panel reads “Cluster access denied,” names the affected cluster
and failed read, then offers “Open source settings” and an appropriate retry or
agent instruction. Unknown errors use a diagnostic reference, not a guessed fix.

## Tokens and component rules

- Canvas #f6f7fb; surface #ffffff; text #172033; secondary #566176.
- Border #dce2eb; primary #4f46e5; primary hover #4338ca.
- Success #177245; failure #b42318; warning #925400; info #3159ad.
- Status backgrounds are subtle tints with high-contrast text; verify contrast.
- Spacing scale: 4, 8, 12, 16, 24, 32, 48px. Panel radius: 12px.
- System sans-serif initially; 14–16px body, 12–13px metadata, 28–32px page titles.
- Buttons and controls have visible focus rings and at least 44px touch targets.
- Mobile replaces the sidebar with a keyboard-accessible drawer; tables become
  compact card/list layouts where horizontal columns would hide key actions.
- Copy leads with achievement or next action: “Added 3 evidence records,”
  “Review 2 requirements,” “Refresh AWS sign-in.” Avoid repeated generic caveats.

Build a fixture gallery for every component/state and inspect screenshots at
390px, 768px and 1440px before integrating domain data.
