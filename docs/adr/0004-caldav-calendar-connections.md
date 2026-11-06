# Phone calendars connect over CalDAV with generated per-device secrets

Members see and edit their Appointments and Tasks in the iPhone's built-in Calendar and Reminders apps because Homebase acts as a CalDAV server. Each Calendar Connection has its own secret that Homebase generates. On iOS, the secret is delivered inside a configuration profile that Homebase serves, so no Member ever types it. We rejected a native app because it needs a paid Apple developer account and a Mac to build on. We also rejected a read-only `.ics` subscription: it covers viewing, but the iPhone apps could not change anything. CalDAV clients only accept a username and password, so the secret travels as one. It is not a sign-in password, and it does not weaken ADR-0002: sign-in stays OAuth-only, and each secret is reset-able, tied to one device, and grants only calendar access.

## Consequences

- The `events` columns `source`, `external_id` and `sync_status` were added at the start as scaffolding for pulling Appointments from an external calendar provider, with Homebase as the client. Nothing writes values other than the defaults. Under this decision Homebase is the server, so they are not needed as designed. Whoever implements CalDAV should either repurpose them, for example to keep the UID a phone assigns to an Appointment it creates, or drop them deliberately, rather than leave them half-used.
- Homebase Appointments do not repeat, so the server rejects repeating events created on a device.
- Alerts set on a device are discarded. Reminders stay Homebase's job.
