# ENIE project update notes

## Changes in this update
- Added the generated ENIE logo to shared site headers, account navigation, and the login/signup experience; also added it as the favicon.
- Redesigned login and signup as responsive split-screen pages with ENIE brand messaging, improved form spacing, clearer headings, and modern button/input states.
- Added a vendor-only **AI Writing Studio** shortcut to the account page.
- Made the account page tolerate missing optional profile columns in older deployments and keep loading if the order summary query fails; added detailed server-side diagnostics.
- Improved session handling so an expired/revoked session is cleared and browser fetch requests receive a JSON `401` instead of silently following a redirect to the login page.
- Made customer-message AI draft errors return JSON, with clearer quota/configuration/session messages.
- Separated vendor AI and customer-chat AI rate limits and increased the allowance to 20 requests per 15 minutes per IP for each feature.
- Added OpenAI support using `OPENAI_API_KEY` and `OPENAI_MODEL`, matching the project's `.env.example`; retained Gemini support through `GEMINI_API_KEY` and `GEMINI_MODEL`. OpenAI is preferred when both are configured.
- Applied shared responsive UI refinements across the site's cards, forms, buttons, navigation, tables, and focus states.

## Configuration reminder
The AI service requires a valid provider key in the server environment:
- OpenAI: `OPENAI_API_KEY` (optional `OPENAI_MODEL`, defaults to `gpt-4o-mini`)
- Gemini: `GEMINI_API_KEY` (optional `GEMINI_MODEL`)

Do not put real keys in the source repository or browser JavaScript. Configure them in the local `.env` or hosting provider's environment settings.

## Testing limitation
The Go files were formatted with `gofmt`. Automated `go test ./...` could not run in this environment because the project requests Go 1.26 and the environment could not download the toolchain or required Go modules due to unavailable network access. Please run the test suite in your normal development environment before deploying.
