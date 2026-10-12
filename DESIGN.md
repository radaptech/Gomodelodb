---
name: brModelo (gobrmodelo)
description: An ER/relational modeling editor set as a page of a Brazilian ABNT monograph, corrected in the advisor's red pen.
colors:
  caneta: "#b3141d"
  marca: "#b3141d14"
  papel: "#ffffff"
  mesa: "#d4d6d9"
  tinta: "#171717"
  grafite: "#555555"
  filete: "#17171733"
typography:
  display:
    fontFamily: "Liberation Serif, Times New Roman, serif"
    fontSize: "24px"
    fontWeight: 700
    letterSpacing: "normal"
  title:
    fontFamily: "Liberation Serif, Times New Roman, serif"
    fontSize: "15px"
    fontWeight: 700
  body:
    fontFamily: "Liberation Serif, Times New Roman, serif"
    fontSize: "15px"
    fontWeight: 400
  caption:
    fontFamily: "Liberation Serif, Times New Roman, serif"
    fontSize: "13px"
    fontWeight: 400
  label:
    fontFamily: "Liberation Serif, Times New Roman, serif"
    fontSize: "13px"
    fontWeight: 400
  source:
    fontFamily: "Liberation Serif, Times New Roman, serif"
    fontSize: "11px"
    fontWeight: 400
  figure-label:
    fontFamily: "Liberation Sans, Arial, sans-serif"
    fontSize: "12px"
    fontWeight: 400
  figure-name:
    fontFamily: "Liberation Sans, Arial, sans-serif"
    fontSize: "12px"
    fontWeight: 700
  figure-small:
    fontFamily: "Liberation Sans, Arial, sans-serif"
    fontSize: "11px"
    fontWeight: 400
rounded:
  none: "0px"
spacing:
  hairline-pad: "6px"
  cell-pad: "10px"
  sheet-gap: "18px"
  sheet-margin: "22px"
  number-column: "44px"
  figure-margin: "72px"
components:
  sheet:
    backgroundColor: "{colors.papel}"
    rounded: "{rounded.none}"
    padding: "22px"
  sheet-side:
    backgroundColor: "{colors.papel}"
    width: "300px"
    padding: "22px"
  sumario-entry:
    textColor: "{colors.tinta}"
    typography: "{typography.body}"
    height: "30px"
  sumario-entry-primary:
    textColor: "{colors.tinta}"
    typography: "{typography.title}"
    height: "30px"
  sumario-entry-hover:
    backgroundColor: "{colors.marca}"
    textColor: "{colors.caneta}"
  sumario-entry-active:
    textColor: "{colors.caneta}"
  ficha-field:
    backgroundColor: "{colors.papel}"
    textColor: "{colors.tinta}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "6px 0"
  ficha-field-label:
    textColor: "{colors.grafite}"
    typography: "{typography.label}"
    width: "44px"
  margin-note:
    textColor: "{colors.caneta}"
    typography: "{typography.body}"
---

# Design System: brModelo (gobrmodelo)

## Overview

**Creative North Star: "The Corrected Monograph"**

The editor is a page of a Brazilian TCC set to ABNT: white A4 sheets lying on a cool grey desk, black ink, a Times-metric serif for everything the document says and an Arial-metric sans for everything inside a figure. There is no toolbar and no inspector. Tools and commands are a SUMÁRIO with dot leaders whose page-number column holds the keyboard shortcut. Properties are a ruled catalog card, the Ficha. The diagram is a numbered figure with its caption above and its Fonte line below. Errors are what an advisor would write: a red-pen loop around the shape and an italic remark in the margin.

The only voice that is not ink is the advisor's red pen. It marks where you are (active SUMÁRIO number and tick), what the pointer is over, what is selected, the foreign keys the conversion added, and everything that went wrong. Density is a printed page: generous margins, a 30px line rhythm in the SUMÁRIO, hairline rules instead of boxes.

The world is light only, because it is paper. It rejects the category-default diagram editor (icon toolbar and grey inspector panels) and the dark neon developer tool.

**Key Characteristics:**
- Two sheets on a desk: a 300px SUMÁRIO sheet and a wide figure sheet, each with a soft layered shadow.
- Serif for the document, sans for the figure; never swapped.
- One accent, the red pen, used as correction and state, never as decoration.
- Rules, not boxes: heavy ink rules open and close cards and tables, hairlines separate rows.
- Square corners on every surface and field.

## Colors

Ink on paper over a cool grey desk, with a single red pen.

### Primary
- **Advisor's Red Pen** (caneta): Active SUMÁRIO number and its 10x2px tick, hovered entry label, selected shapes and links (stroke 2px), marquee outline, red-pen loop and margin note, error card text, FK key markers in logical tables, converter "Nota:" lines. Also the Fyne Primary, Focus, Hyperlink and Error color. 6.6:1 on paper.
- **Red-Pen Wash** (marca): The 8% red fill under a hovered SUMÁRIO entry and inside the marquee selection. Fyne Hover color. Stronger washes of the same red serve Fyne Pressed (15%) and text Selection (22%).

### Neutral
- **Paper** (papel): Every sheet, the figure canvas, input and menu backgrounds, the fill inside entity rectangles and table boxes, the scrap behind a cardinality label.
- **Desk Grey** (mesa): The window background the sheets lie on. Never used on a sheet.
- **Ink** (tinta): All primary text, shape strokes (1.25px), heavy rules (1.75px), field underlines (1px).
- **Graphite** (grafite): Secondary text: dot leaders, shortcuts, the cover year, field labels, column heads, data types, the Fonte line, empty-state guidance, placeholders. 7.4:1 on paper.
- **Hairline** (filete): Ink at 20% for the 0.75px row rules of the Ficha and Fyne separators. Scrollbars are ink at 15%, 3px wide on a transparent track.

### Named Rules
**The Red Pen Rule.** Red means the advisor touched it: state (active, hover, selected) or correction (errors, notes, FKs the converter added). Nothing is red for decoration, and no second accent exists.

**The Paper Is Light Rule.** There is no dark variant. The theme forces the light Fyne variant for every fallback color.

## Typography

**Document Font:** Liberation Serif (Times New Roman metrics), embedded Regular, Bold, Italic
**Figure Font:** Liberation Sans (Arial metrics), embedded Regular, Bold
**Mono:** Fyne default, not used visibly

**Character:** The pairing ABNT itself prescribes: Times for the text, Arial inside figures. Both ship embedded under the OFL so the page renders identically everywhere.

### Hierarchy
- **Display** (serif bold, 24px): The cover title "BRMODELO", centered, once per window.
- **Title** (serif bold, 15px): Section level. Primary SUMÁRIO entries in caps (ABNT NBR 6027), the centered "SUMÁRIO" heading, the Ficha heading.
- **Body** (serif regular, 15px): Secondary SUMÁRIO entries, shortcuts, field values. Fyne text size is 15.
- **Remark** (Body in the italic cut, 15px): Cover subtitle, Ficha notes, empty-state guidance (graphite), red-pen notes (caneta, wrapped near 32 characters).
- **Caption** (13px): "Figura N –" in serif bold followed by the title in serif regular, above the figure.
- **Label** (serif regular, 13px, graphite): Ficha field labels and the cover year.
- **Source** (serif regular, 11px, graphite): "Fonte: ..." 30px below the figure's bounding box.
- **Figure Label / Figure Name** (sans 12px regular / bold): Attribute names (regular), entity and relationship names (bold, centered). Table names use bold at 13px.
- **Figure Small** (sans 11px): Cardinalities, and in bold graphite the table column heads Campo | Tipo | Chave.

### Named Rules
**The Two Faces Rule.** Serif speaks for the document (SUMÁRIO, Ficha, captions, notes); sans lives only inside a figure. A figure label in serif or a SUMÁRIO line in sans breaks the world.

**The Weight Is Level Rule.** Bold means hierarchy level (primary section, figure number, entity name, PK), never state. Active and hover change color, never weight.

## Layout

A window of 1200x760 holds a desk with an 18px horizontal and 22px vertical gutter. On the left, a 300px sheet with a 22px inner margin; 18px of desk; then the figure sheet filling the rest with no inner margin, its scrollable canvas being the page itself.

The left sheet stacks the cover block (title, italic subtitle, year, 16px below), the centered SUMÁRIO heading, the entries, and the Ficha pinned at the foot. When they do not fit, the sheet scrolls rather than overlapping.

SUMÁRIO entries are 30px tall. Numbers start at 16px, labels at the 44px number column so "1" and "6.1" labels align, the shortcut is right-aligned 6px from the edge, and the dot leader starts on a fixed grid so dots line up vertically across entries.

On the figure sheet, the caption sits about 44px above the figure's bounding box and the Fonte line 30px below it; both follow the drawing. Empty states and the logical figure use a page margin (72px for empty-state text, 60px left / 112px top for laid-out tables). Ficha rows carry 6px above and below each rule; table cells pad 10px horizontally with 22px rows.

## Elevation & Depth

Depth exists in one place: sheets lie on the desk. Each sheet carries three stacked, slightly widening black rectangles offset 1, 3 and 6px down at 5-9% opacity, which read as a soft blurred shadow. Everything on a sheet is flat ink; nothing on the page lifts.

### Shadow Vocabulary
- **Sheet on desk** (three layers: offset 6px at 5%, 3px at 6%, 1px at 9% black, each widened by its offset): Every folha, both windows.
- **Fyne overlay shadow** (black at 18%): Theme fallback for popups.

### Named Rules
**The Flat Page Rule.** Only sheets have shadows. Cards, tables, fields and shapes are drawn on the paper, not stacked above it.

## Shapes

Paper has square corners: input, selection and scrollbar radii are 0. Structure is drawn with rules rather than enclosed boxes; the Ficha and logical tables have no side borders. The ER notation is outline-only ink at 1.25px: entity rectangle, relationship diamond from four lines, attribute 10px circle filled only when it is an identifier. Selection thickens the stroke to 2px and turns it red. The one round form is handwriting: the red-pen loop is a 1.5px pill (radius half its height) drawn around a shape.

## Components

### SUMÁRIO entry (navigation and tool picker)
The table of contents is the toolbar. "3 RELACIONAMENTO . . . . . R".
- **Primary** (whole number): bold caps label and number. **Secondary** (dotted number like 6.2): regular, sentence case.
- **Leader and shortcut:** graphite dots on the shared grid, shortcut right-aligned in the page-number column.
- **Hover:** red-pen wash across the row, label turns red, pointer cursor.
- **Active:** number turns red and a 10x2px red tick appears in the left margin. Weight never changes.
- **Heading line:** an entry with no action (e.g. "6 ARQUIVO") keeps the number column and rhythm but has no leader, no hover and the default cursor.

### Ficha (properties card)
Ruled like a library catalog card.
- **Rules:** 1.75px ink rule on top, the bold title as first row, 0.75px hairline under each row, a 1px ink rule under each input row, and a 1.75px ink rule closing it.
- **Background:** the sheet itself; no fill, no border, no shadow.
- **Error variant:** titled "Ficha – Atenção" with the message as a red italic note.

### Inputs / Fields
- **Style:** borderless, paper background, square; the card's 1px ink rule is the only frame. A 13px graphite label sits in a 44px column in front of the value, aligned with the card's text edge.
- **Focus:** Fyne focus color is the red pen.

### Figure (signature)
The diagram is a captioned ABNT figure: "Figura 1 – Modelo conceitual" above, "Fonte: elaborado pelo autor (ano)." below, both tracking the drawing's bounds. Cardinalities sit on a scrap of paper so no line strikes through them, drawn under the shapes. The empty state keeps caption and Fonte and puts italic graphite guidance where the figure will be.

### Logical table (signature)
IBGE/ABNT table style: bold sans name above, 1.75px top rule, Campo | Tipo | Chave heads in small bold graphite, 0.75px rule under the heads, rows with no rules, 1.75px closing rule, no verticals. PK in bold ink, FK in bold red. Paper fill hides links behind the table; cardinalities step outside the box. "sem campos" in italic graphite for an empty table.

### Red-pen margin note (feedback)
Errors and prompts are never dialogs. The advisor circles the shape in a red pill loop and writes a 15px italic red remark beside it; transient notes fade after a few seconds. Marquee selection is a red-pen wash with a 1px red outline.

## Do's and Don'ts

### Do:
- **Do** put every new command into the SUMÁRIO as a numbered entry with its shortcut in the page-number column.
- **Do** keep serif for document text and sans only inside figures.
- **Do** use the red pen (caneta) only for state and correction: active, hover, selected, error, FK, note.
- **Do** draw structure with ink rules (1.75px heavy, 0.75px hairline) instead of boxes.
- **Do** give every figure an ABNT caption above and a Fonte line below that follow its bounding box.
- **Do** report errors as red italic notes on the Ficha or in the figure margin.

### Don't:
- **Don't** add an icon toolbar, grey inspector panel, or modal dialogs for errors.
- **Don't** add a dark theme or a second accent color.
- **Don't** change weight to show state; bold is section level.
- **Don't** round the corners of sheets, fields, cards or tables; the pen loop and attribute circles are marks, not containers.
- **Don't** put shadows on anything that is not a sheet.
