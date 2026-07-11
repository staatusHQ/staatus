# Staatus GitHub Action

This directory is reserved for the future GitHub Action wrapper.

The intended flow is:

1. Run configured checks.
2. Append repo-friendly history data.
3. Render static public API JSON.
4. Let the user's chosen deploy step publish the static site.

The core CLI is kept separate so humans, scripts, and agents can run the same commands locally.
