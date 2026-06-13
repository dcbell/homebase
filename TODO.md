# TODO

## Frontend Maintainability

- Continue extracting repeated server-rendered frontend markup into reusable templates or helpers.
- Initial pass extracted standard page headers and title action menus.
- Remaining candidates include index list cards, right-side info panels, attachment lists, search/filter rows, and common modal shells.
- Consolidate repeated modal patterns where practical, including create/edit forms, attach/link modals, and info/preview modals.
- Keep the refactor behavior-neutral and prioritize changes that make future UI work easier to manage.

## API Documentation

- Generate an OpenAPI/Swagger document for the API.
- Publish the Swagger document somewhere accessible for reference by developers and future agentic integrations.
- Keep the API document updated as endpoints, payloads, and auth requirements change.

## Authentication

- Replace the current Google-specific authentication flow with generic OAuth/OIDC support.
- Make the OAuth provider configurable so it can point at an internal OAuth provider.
- Update login, callback, logout, session setup, and documentation around the generic provider configuration.

## Archive Recovery

- Add archived-item views and recovery actions across modules.
- Include archived projects, project folders, standalone tasks, project tasks, documents, contacts, assets, routines, lists, list items, and asset maintenance items.
- Make archived views reachable from each relevant index/detail workflow without cluttering the primary active-item screens.
