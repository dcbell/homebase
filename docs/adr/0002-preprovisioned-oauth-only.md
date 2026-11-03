# Sign-in is OAuth/OIDC only, for pre-provisioned Members

Homebase has no local passwords and no self-signup. A sign-in succeeds only when the OAuth identity's email has already been added as a Member by an Owner. The first Owner is created from the `BOOTSTRAP_OWNER_*` environment variables. We chose this over open registration or built-in password accounts because Homebase is a self-hosted, single-household tool. Identity is delegated to the household's existing provider, and nobody outside the household can create access. The development-only demo login is not permitted in production.
