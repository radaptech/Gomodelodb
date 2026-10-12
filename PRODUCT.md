# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

Note: not literally web. A native desktop app built with Go + Fyne v2.8.1 (Linux first, Wayland/X11). None of the iOS/Android native guidance applies; desktop conventions (pointer, keyboard, resizable window) do.

## Users

The author: a developer reverse engineering brModelo (the Brazilian teaching tool for relational database modeling) into Go + Fyne as a personal study and portfolio project. Secondary, implied: anyone who knows the original brModelo and wants to draw a conceptual ER diagram and convert it to a logical one.

## Product Purpose

Draw conceptual (ER) diagrams in the Heuser/Chen notation brModelo teaches, convert them to logical (relational) diagrams with the same rules as brModelo's Java converter, and read/write brModelo's XML files. Success: the port behaves like brModelo where it counts (conversion, file format) while feeling like a modern, well-made desktop tool, good enough to show in a portfolio.

## Positioning

A faithful port of brModelo's conversion logic and file format (interoperable with the Java app's .xml files), not a generic diagram editor.

## Operating Context

Single user at a desktop, mouse and keyboard. Two windows: the conceptual editor (main) and the logical diagram produced by "Gerar lógico". Files are brModelo XML (`tpConceitual`, `tpLogico`); `.brM3` (Java serialization) is not supported.

## Capabilities and Constraints

- Conceptual: entidade, relacionamento, atributo (identificador), ligação with cardinality (1,1)/(0,1)/(1,n)/(0,n); entity-to-entity link creates a relationship; self-relationship; attribute tool on a shape attaches it; area selection and group move; properties panel on the left.
- Logical: tables, fields (PK/FK), FK constraints with ON DELETE, Java's table organizer, centered on open.
- SQL migration from the logical model for PostgreSQL, MySQL or SQL Server.
- MCP server (streamable HTTP, 127.0.0.1:7421/mcp, only while the app is open): Claude edits the open diagram live through the same use cases as the mouse.
- Undo/redo (Ctrl+Z / Ctrl+Y) for every change, mouse, MCP or assistant; one assistant request is one step. Zoom 25–300 % (Ctrl+= / Ctrl+- / Ctrl+0 and an on-sheet control) on both figures.
- In-app ORIENTADOR: chat with Claude, GPT, Gemini (API keys in the system keyring) or local Ollama, driving the same MCP tools.
- UI language: Brazilian Portuguese.
- Rendering is a custom Fyne canvas (rectangles, lines, circles, text); Fyne has no polygon fill, so shapes are built from those primitives.
- Not ported yet: especialização, união, entidade associativa, other brModelo diagram types, printing.

## Brand Commitments

Name "brModelo" (original by sis4.com, open source). The user gave total freedom over the look, including the shape styling of the notation, as long as the notation stays readable as ER.

## Evidence on Hand

Original Java source in `../src`, sample diagrams embedded in `../Template.brMt`. No screenshots, users, or testimonials; do not invent any.

## Product Principles

- Conversion and file compatibility with brModelo are the truth; the UI may change freely.
- The diagram is the work: chrome stays out of its way.
- Direct manipulation first: click to create, drag to move, edit in place on the side panel.
