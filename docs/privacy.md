# Privacy

Specfork keeps as little as it can. This page says what, and how to remove it.

## What is stored

- When you sign in with GitHub: your numeric GitHub user ID, your login and the time of your last sign-in. Specfork asks GitHub only for `read:user`, uses the token once and does not keep it.
- Your scores: one score from 1 to 5 per spec, and when you last changed it.
- Daily usage counters of the hosted MCP server: day, tool and spec name, with a count. They cannot be traced to a person.

## What is not stored

No email address, no password, no GitHub token, no IP address, no user agent, no search text and no spec text. Server logs hold the route, status and duration of a request, nothing that names you.

## Delete your account

Signed in, use "Sign out" to end the session. To delete your account and every score you gave, send `DELETE https://api.specfork.dev/me` with the header `X-Specfork: 1` while signed in, or open an issue on [specfork/specfork](https://github.com/specfork/specfork/issues). Pull requests that were already opened for a claim stay on GitHub.
