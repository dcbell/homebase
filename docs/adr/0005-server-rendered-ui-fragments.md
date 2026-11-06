# Interactive web UI is rendered by the server; scripts only swap fragments

When part of a page has to change as someone edits it, such as a Reminder Times frame's summary or which of its boxes apply, `cmd/web` renders the new markup and the page's script fetches it and swaps it in. The script never works out wording, which controls apply, or what a value means. We chose this over computing them in the browser after #57: the script there re-implemented the server's summary and box-visibility rules, drifted from them, and showed the wrong Reminder Time while a hidden box quietly submitted another. With one renderer, wording and rules can't diverge, and restyling the markup can't break logic. The cost is a request per meaningful edit, and no refresh while offline. Without JavaScript the page still works: the server renders every control, and form parsing ignores the ones that don't apply. This is about rendering. Business rules stay in the API, as ADR-0001 requires.

## Consequences

- The server marks controls that don't apply to a row's current shape (for example `data-applies="false"`), and CSS hides them only when the script is running. The script never sets that visibility itself.
- Logic is tested in Go against the rendered markup. Scripts are thin enough not to need a JavaScript test harness.
