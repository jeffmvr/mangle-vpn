# Mangle VPN — web interface

Vue 3, Vite and Tailwind 4. It is compiled into the Go binary, so there is
nothing to deploy separately.

## Prerequisites

- Node 20 or newer (`node --version`)
- Go 1.27 or newer, for the backend this talks to

## Working on it

From `mangle-go`:

```bash
make dev
```

That builds this interface and the server, provisions `../data` the first
time, and serves both at **http://localhost:9443**. It rebuilds and restarts
on every save to the Go code or to `ui/`; refresh the browser to see a
change.

The first visit redirects to the setup wizard. Its setup code is in
`../data/keys/setup.token`. Go through it, then enrol an authenticator when it
asks.

### With Vite's dev server

For instant reloads while working only on the interface, run the backend on
its own and Vite in front of it:

```bash
make run                            # in mangle-go: the backend at :9443
cd ui && npm install && npm run dev # in another terminal
```

Open **http://localhost:5173**. Vite serves the application shell and
proxies everything the backend renders itself (`/api`, `/login`, `/mfa`,
`/install`, `/password`, `/logout`, `/oauth`, `/logo`) through to port 9443,
so you work against real data rather than a mock. Always reach it through
localhost:5173 then, never :9443 directly: cookies are scoped per host, so
signing in on one leaves you signed out on the other.

The backend runs with `-insecure` in development. Served over TLS it marks
its session cookie `Secure`, and a browser will not send that to
`http://localhost`, so you would appear to be signed out on every request.

## Building

```bash
make build                          # in mangle-go: this, then the binary that embeds it
```

`npm run build` on its own writes into `../internal/webui/dist`, the
directory the binary embeds: `index.html` is the shell, answered at `/`, and
`static/` holds the assets, served at `/static`. Bundles have a content hash
in their names and are cached by browsers for good; the shell is
revalidated on every load, so an upgrade takes effect on the next visit.

## What is here

My devices, and in administration the connected clients, users, groups,
activity, logs and settings. The sign in, two-factor, password and setup
pages are rendered by Go, from `../internal/web/templates`, and share this
interface's look.

## Layout

```
src/
  assets/app.css        design tokens, in one @theme block
  lib/api.js            axios instance, CSRF, what to do when refused
  lib/                  formatting, the settings and log sections, activity labels
  stores/session.js     the signed in user and server state
  router/               routes, hash based
  components/ui/        the shared primitives
  layouts/              the user and admin shells
  views/                one file per screen
```

### Two things worth knowing before editing

**One accent, and colour otherwise means state.** Blue marks what can be
acted on and where you are. Green is connected, amber wants attention, red
is destructive. The tokens live in one `@theme` block in `app.css`, and the
Go-rendered pages copy the same values into `templates/Layout.html`, so a
change to one belongs in the other.

**The CSRF contract is fixed.** The server issues a readable `csrftoken`
cookie and expects it echoed in the `X-CSRFToken` header. `lib/api.js` does
this on every request; changing it breaks every write.
