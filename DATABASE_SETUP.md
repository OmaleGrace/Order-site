## AI vendor writing assistant

Set `OPENAI_API_KEY` in `.env` or the deployment environment to enable the
optional vendor writing assistant at `/vendor/ai`. `OPENAI_MODEL` is optional
and defaults to `gpt-4o-mini`. The tool drafts menu descriptions and social
captions only; vendors must review the result and publish it themselves. Never
expose the key in frontend JavaScript or commit it to source control.
