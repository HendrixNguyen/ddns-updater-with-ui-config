# Web UI and REST API

The ddns-updater program serves a web UI and a JSON REST API from the same HTTP
server. Everything below is served by the program itself; there is no separate
service to install.

- [Base URL](#base-url)
- [Authentication](#authentication)
- [Common conventions](#common-conventions)
- [Status codes](#status-codes)
- [Endpoints](#endpoints)
  - [GET /](#get-)
  - [GET /settings](#get-settings)
  - [GET /update](#get-update)
  - [GET /static/*](#get-static)
  - [GET /api/providers](#get-apiproviders)
  - [GET /api/settings](#get-apisettings)
  - [POST /api/settings](#post-apisettings)
  - [PUT /api/settings](#put-apisettings)
  - [PUT /api/settings/{index}](#put-apisettingsindex)
  - [DELETE /api/settings/{index}](#delete-apisettingsindex)
  - [POST /api/settings/validate](#post-apisettingsvalidate)
  - [POST /api/reload](#post-apireload)
  - [GET /api/records](#get-apirecords)
- [Examples](#examples)
- [Settings file format](#settings-file-format)
- [Hot reload semantics](#hot-reload-semantics)

## Base URL

All the paths in this document are relative to `ROOT_URL`, which is configured
with the `ROOT_URL` environment variable (or the `--root-url` flag) and defaults
to `/`.

- With the default `ROOT_URL=/`, the API is served at `http://<LISTENING_ADDRESS>/api/...`,
  for example `http://localhost:8000/api/settings`.
- With `ROOT_URL=/ddns`, every path gains that prefix: the settings page is at
  `http://localhost:8000/ddns/settings` and the API at
  `http://localhost:8000/ddns/api/settings`.

`ROOT_URL` is normalized by removing its trailing slash, so `ROOT_URL=/` behaves
like `ROOT_URL=` (empty). Requesting the `ROOT_URL` path itself without its
trailing slash (`GET /ddns` when `ROOT_URL=/ddns`) answers with a
`308 Permanent Redirect` to `ROOT_URL/`.

The examples in this document use the default `ROOT_URL=/` and the default
`LISTENING_ADDRESS=:8000`, i.e. a base URL of `http://localhost:8000`. Add your
`ROOT_URL` prefix to all of them if you changed it.

## Authentication

**There is none.** The web UI and the API are unauthenticated: there is no
login, no API key, no token and no per-user permission. Anyone able to open a
TCP connection to `LISTENING_ADDRESS` can read and rewrite your DNS provider
credentials and repoint your records.

Do not expose the listening address to the public internet. If you need remote
access, run the program on a private network and put a reverse proxy with
authentication in front of it.

## Common conventions

- Request and response bodies are JSON, encoded in UTF-8.
- Every request **with a body** must send `Content-Type: application/json`,
  otherwise the answer is `415`. Parameters such as `; charset=utf-8` are
  accepted.
- Request bodies are limited to **1 MiB** (1048576 bytes). A larger body is
  rejected with `413`.
- The body must contain a single JSON value. An empty body is `400`, and so is a
  malformed or truncated body.
- Errors are always a JSON object with a single `errors` array of strings, one
  entry per line of the underlying error:

  ```json
  { "errors": ["setting 0: unknown provider: duckdnss"] }
  ```

  When several entries are invalid, every problem is reported at once, prefixed
  by the zero-based index of the offending entry, for example
  `setting 1: validating provider specific settings: token is not valid: ...`.
  The wording after the index comes from the provider itself, so it differs from
  one provider to the next.
- `GET` endpoints ignore a request body.
- Successful responses of the JSON API set `Content-Type: application/json;
  charset=utf-8`. `GET /update` is the exception, it answers in plain text.

## Status codes

| Code | Name | When it is returned |
| --- | --- | --- |
| 200 | OK | Every successful read (`GET /api/...`, `GET /settings`, `GET /static/*`) and every successful mutation except an addition (`PUT ...`, `DELETE ...`, `POST /api/reload`). `POST /api/settings/validate` also always answers `200`, even when the document is invalid: the verdict is in the body. |
| 201 | Created | `POST /api/settings` added a settings entry. |
| 202 | Accepted | `GET /update` ran an update cycle for all the records and they all succeeded. |
| 308 | Permanent Redirect | `GET {ROOT_URL}` (no trailing slash) when `ROOT_URL` is not `/`. |
| 400 | Bad Request | The `{index}` URL parameter is missing, is not a number, is out of range as an integer or is negative; or the JSON body is empty, malformed or truncated. |
| 404 | Not Found | The `{index}` URL parameter is a valid non-negative integer but no settings entry has that index; or the requested path (or static file) does not exist. |
| 405 | Method Not Allowed | The path exists but was requested with an HTTP method the route does not serve. |
| 413 | Request Entity Too Large | The JSON body is larger than 1 MiB. |
| 415 | Unsupported Media Type | The `Content-Type` header is missing or is not `application/json`, on a request with a body. |
| 422 | Unprocessable Entity | The JSON body is syntactically valid but the settings it holds are not: the entry is not a JSON object, it produces no DNS provider (missing or unknown `provider`), or a provider rejected one of its settings. The settings file is left untouched. |
| 500 | Internal Server Error | The settings file could not be read or written, the page template could not be rendered, or (for `GET /update`) at least one record failed to update. |

## Endpoints

### GET /

The records dashboard, an HTML page. It renders the records server side, then
`static/app.js` refreshes it from `GET {ROOT_URL}/api/records` every 10 seconds.

- Request body: none
- Response: `text/html; charset=utf-8`
- Status codes: `200`; `500` if the page template cannot be rendered

### GET /settings

The configuration page, an HTML page. It is a shell only: every piece of content
is built in the browser from `GET {ROOT_URL}/api/providers` and
`GET {ROOT_URL}/api/settings`, and every change is sent back to the JSON API.
JavaScript is required to use it, otherwise the page shows a notice and you must
edit the settings file directly.

- Request body: none
- Response: `text/html; charset=utf-8`
- Status codes: `200`; `500` if the page template cannot be rendered

### GET /update

Runs an update cycle for all the records immediately, without waiting for the
next period, and blocks until it is finished. The record statuses on the
dashboard reflect the outcome.

- Request body: none
- Response on success: `202 Accepted` with the plain text body
  `All records updated successfully in 2.1s`
- Response on failure: `500` with `{"errors":["<error>", "..."]}` carrying the
  errors reported by the update cycle, for example the records that failed
- Status codes: `202`, `500`

### GET /static/*

Serves the static assets of the web UI: `app.js`, `styles.css`, `favicon.svg`
and `favicon.ico`. The `static/` prefix is stripped before the file lookup, so
`GET {ROOT_URL}/static/app.js` serves `ui/static/app.js`.

- Request body: none
- Response: the file content, with the media type derived from its extension
- Status codes: `200`; `404` if the file does not exist

### GET /api/providers

Lists every DNS provider the updater supports, sorted by name, together with the
settings entry fields that provider accepts, so a client can build a form for it.

- Request body: none
- Response body:

  ```json
  {
    "providers": [
      {
        "name": "cloudflare",
        "label": "Cloudflare",
        "doc": "docs/cloudflare.md",
        "fields": [
          {
            "key": "zone_identifier",
            "label": "Zone Identifier",
            "type": "text",
            "required": true,
            "help": "Zone ID of the zone to update, as shown in the Cloudflare dashboard",
            "placeholder": ""
          },
          {
            "key": "token",
            "label": "API Token",
            "type": "password",
            "required": false,
            "help": "Scoped API token with Zone:DNS:Edit permission",
            "placeholder": "cf_..."
          }
        ]
      }
    ]
  }
  ```

  | Field | Type | Description |
  | --- | --- | --- |
  | `providers` | array | One object per supported provider, sorted by `name`. |
  | `providers[].name` | string | The value to use in the `provider` key of a settings entry, for example `cloudflare`. |
  | `providers[].label` | string | The provider name in title case, for display. |
  | `providers[].doc` | string | The repository relative path of the provider documentation page, for example `docs/cloudflare.md`. |
  | `providers[].fields` | array | The provider specific keys of a settings entry. |
  | `providers[].fields[].key` | string | The key to use in the settings entry object. |
  | `providers[].fields[].label` | string | The human readable label of the field. |
  | `providers[].fields[].type` | string | `text`, `password`, `number`, `checkbox` or `textarea`. It tells a client which form control to render. |
  | `providers[].fields[].required` | boolean | Whether the provider rejects the entry when this key is missing or empty. |
  | `providers[].fields[].help` | string | A one line explanation of what to put in the field. |
  | `providers[].fields[].placeholder` | string | An example value, possibly an empty string. |

  The keys common to every settings entry (`provider`, `domain`, `owner`,
  `ip_version`, `ipv6_suffix`, and the retro-compatible `host` and `provider_ip`)
  are not listed in `fields`. Providers with no
  curated field list (for example `custom`) are returned with an empty `fields`
  array, which means the client must fall back to a free-form JSON editor.

- Status codes: `200`

### GET /api/settings

Returns the settings entries currently in the settings file, plus the metadata
needed to know which source is actually in effect. It never writes anything.

- Request body: none
- Response body:

  ```json
  {
    "settings": [
      {
        "provider": "duckdns",
        "domain": "sub.duckdns.org",
        "token": "00000000-0000-0000-0000-000000000000",
        "ip_version": "ipv4"
      }
    ],
    "warnings": [],
    "source": "file",
    "env": "unset",
    "filePath": "/updater/data/config.json",
    "records": 1
  }
  ```

  | Field | Type | Description |
  | --- | --- | --- |
  | `settings` | array | The settings entries, in file order, as raw JSON objects. It is `[]` and never `null` when there is none. The `settings[0]` entry is the one `PUT /api/settings/0` and `DELETE /api/settings/0` act on. |
  | `warnings` | array of strings | Non-fatal problems found while reading the settings, such as a retro-compatible key being used. It is `[]` and never `null`. |
  | `source` | string | Where the settings in use come from: `file`, `env` or `empty`. `empty` means there is no settings entry at all. See [Configuration precedence](../README.md#configuration-precedence-and-restart-safety). |
  | `env` | string | `set` when the `CONFIG` environment variable has a non-empty value, `unset` otherwise. |
  | `filePath` | string | The absolute path of the settings file, which is the file the mutating endpoints write to. |
  | `records` | integer | The number of in-memory DNS records the settings currently produce, one per domain and owner. It is larger than the number of entries when an entry lists several domains, for example `"domain": "sub.duckdns.org,other.duckdns.org"`. |

- Status codes: `200`; `500` if the settings file cannot be read

### POST /api/settings

Appends **one** settings entry to the settings file and hot reloads the
resulting records.

- Request body: a single settings entry object, i.e. the contents of one element
  of the `settings` array, **not** wrapped in a `settings` key:

  ```json
  {
    "provider": "duckdns",
    "domain": "sub.duckdns.org",
    "token": "00000000-0000-0000-0000-000000000000",
    "ip_version": "ipv4"
  }
  ```

  Which keys are accepted depends on the provider; see the documentation of your
  provider in [the provider list](../README.md#configuration) and the `fields`
  of `GET /api/providers`.

- Response body: the [settings response](#get-apisettings) shape, with the new
  full list of entries
- Status codes: `201`; `400`; `413`; `415`; `422`; `500`

### PUT /api/settings

Replaces **every** settings entry with the ones given, and hot reloads the
resulting records. If a single entry is invalid, nothing is written and the
`422` response reports every problem at once.

- Request body: a settings document wrapper. Other top level keys are ignored,
  so you may send a copy of the file as it is:

  ```json
  {
    "settings": [
      {
        "provider": "duckdns",
        "domain": "sub.duckdns.org",
        "token": "00000000-0000-0000-0000-000000000000",
        "ip_version": "ipv4"
      }
    ]
  }
  ```

  A missing or `null` `settings` key is treated as an empty list, which removes
  every record.

- Response body: the [settings response](#get-apisettings) shape
- Status codes: `200`; `400`; `413`; `415`; `422`; `500`

### PUT /api/settings/{index}

Replaces the settings entry at the zero-based position `{index}` and hot reloads
the resulting records. The entry is validated before the file is written, so an
invalid entry leaves the file untouched.

- Request body: a single settings entry object, same shape as
  [POST /api/settings](#post-apisettings)
- Response body: the [settings response](#get-apisettings) shape
- Status codes: `200`; `400` if `{index}` is not a non-negative integer; `404` if
  no entry has that index; `413`; `415`; `422`; `500`

### DELETE /api/settings/{index}

Removes the settings entry at the zero-based position `{index}` and hot reloads
the remaining records. The record is stopped being updated, but its history in
`updates.json` is left in place.

- Request body: none
- Response body: the [settings response](#get-apisettings) shape
- Status codes: `200`; `400`; `404` if no entry has that index; `500`

### POST /api/settings/validate

Checks settings entries **without saving anything**, so a client can show inline
feedback before the user commits. It is the same validation the mutating
endpoints run, using the same code path as the program start.

- Request body: either a single settings entry object, or a settings document
  wrapper `{"settings":[...]}`. The form is detected automatically, so the same
  payload can be checked as a whole document or entry by entry.
- Response body:

  ```json
  {
    "valid": false,
    "warnings": [],
    "errors": ["setting 1: unknown provider: duckdnss"],
    "recordCount": 1
  }
  ```

  | Field | Type | Description |
  | --- | --- | --- |
  | `valid` | boolean | `true` when every entry produced a DNS provider, `false` otherwise. |
  | `warnings` | array of strings | Non-fatal problems, returned whether or not the document is valid. It is `[]` and never `null`. |
  | `errors` | array of strings | One entry per problem, prefixed by the zero-based index of the offending entry. It is `[]` and never `null`. |
  | `recordCount` | integer | The number of records **currently running**, not the number the submitted entries would produce. It is there to tell a client that nothing was applied. |

- Status codes: `200` always, for both a valid and an invalid document; `400`;
  `413`; `415`

### POST /api/reload

Re-reads the settings configuration and applies it to the running updater,
**without saving any change**. It is what you want after editing the settings
file by hand, or by a tool other than the API. It is also what the settings file
synchronization does at start up.

- Request body: none
- Response body:

  ```json
  {
    "reloaded": true,
    "records": 1,
    "warnings": []
  }
  ```

  | Field | Type | Description |
  | --- | --- | --- |
  | `reloaded` | boolean | Always `true` in a `200` response. A failure is reported as an error response instead. |
  | `records` | integer | The number of in-memory records after the reload. |
  | `warnings` | array of strings | Non-fatal problems found while reading the settings. It is `[]` and never `null`. |

- Status codes: `200`; `500` if the file cannot be read, or if the settings it
  holds cannot be turned into DNS records (the running records are then left
  untouched)

Note that a `PUT`/`POST`/`DELETE` on the settings endpoints already reloads
internally, so calling `POST /api/reload` right after one of them is redundant.

### GET /api/records

Returns a JSON safe view of every record currently in the database, which is what
the dashboard polls. It never writes anything.

- Request body: none
- Response body:

  ```json
  {
    "records": [
      {
        "domain": "sub.duckdns.org",
        "owner": "sub",
        "provider": "duckdns",
        "ipVersion": "ipv4",
        "status": "success",
        "message": "",
        "currentIP": "203.0.113.5",
        "previousIPs": ["198.51.100.7"],
        "durationSinceSuccess": "2h",
        "lastUpdate": "2024-05-01T10:00:00Z",
        "lastBan": null
      }
    ],
    "count": 1
  }
  ```

  | Field | Type | Description |
  | --- | --- | --- |
  | `records` | array | One object per in-memory record. |
  | `count` | integer | The number of records, equal to the length of `records`. |
  | `records[].domain` | string | The fully qualified domain name, owner included, for example `sub.duckdns.org`. It is the empty string for a record with no provider. |
  | `records[].owner` | string | The owner part of the domain, or `@` for the domain apex. |
  | `records[].provider` | string | The provider name, matching the `provider` key of a settings entry. |
  | `records[].ipVersion` | string | `ipv4`, `ipv6` or `ipv4 or ipv6`. |
  | `records[].status` | string | See [status values](#status-values) below. |
  | `records[].message` | string | The last error or informational message of the record, or the empty string. |
  | `records[].currentIP` | string | The last known public IP address of the record, or the empty string when it is unknown. |
  | `records[].previousIPs` | array of strings | The previously known public IP addresses, most recent first. It is `[]` and never `null`. |
  | `records[].durationSinceSuccess` | string | How long ago the last successful update happened, formatted as `Ns`, `Nm`, `Nh` or `Nd`, or the string `N/A` when the record has no history yet. |
  | `records[].lastUpdate` | string or null | The time of the last successful update, in RFC 3339 format, or `null` when there was never one. |
  | `records[].lastBan` | string or null | The time of the last rate limit ban by the provider, in RFC 3339 format, or `null` when the record was never banned. |

#### Status values

`records[].status` is one of the following five values:

| Value | Meaning |
| --- | --- |
| `success` | The last update cycle pushed a new address to the provider successfully. |
| `failure` | The last update cycle failed, for example because of a bad credential or a rate limit ban. |
| `up to date` | The record already resolves to the current public IP address, so no update was needed. Note the spaces, this is the literal value. |
| `updating` | An update cycle is currently in flight for this record. |
| `unset` | The record has never been updated nor checked yet. |

- Status codes: `200`

## Examples

All the examples assume a base URL of `http://localhost:8000` and a default
`ROOT_URL=/`. They are meant to be copy-pasteable.

### Add a record

```sh
curl -sS -X POST http://localhost:8000/api/settings \
  -H 'Content-Type: application/json' \
  -d '{
        "provider": "duckdns",
        "domain": "sub.duckdns.org",
        "token": "00000000-0000-0000-0000-000000000000",
        "ip_version": "ipv4"
      }'
```

The response is a `201` with the new full list of entries and the metadata,
something like:

```json
{
  "settings": [
    {
      "provider": "duckdns",
      "domain": "sub.duckdns.org",
      "token": "00000000-0000-0000-0000-000000000000",
      "ip_version": "ipv4"
    }
  ],
  "warnings": [],
  "source": "file",
  "env": "unset",
  "filePath": "/updater/data/config.json",
  "records": 1
}
```

### Update it

`{index}` is the zero-based position of the entry in the `settings` array, the
same index the settings page shows in its table.

```sh
curl -sS -X PUT http://localhost:8000/api/settings/0 \
  -H 'Content-Type: application/json' \
  -d '{
        "provider": "duckdns",
        "domain": "other.duckdns.org",
        "token": "11111111-1111-1111-1111-111111111111",
        "ip_version": "ipv4"
      }'
```

### Delete it

```sh
curl -sS -X DELETE http://localhost:8000/api/settings/0
```

### Validate without saving

Check a whole document, here with one valid entry and one missing its token:

```sh
curl -sS -X POST http://localhost:8000/api/settings/validate \
  -H 'Content-Type: application/json' \
  -d '{
        "settings": [
          {
            "provider": "duckdns",
            "domain": "sub.duckdns.org",
            "token": "00000000-0000-0000-0000-000000000000"
          },
          {
            "provider": "duckdns",
            "domain": "other.duckdns.org"
          }
        ]
      }'
```

It answers `200` with `"valid": false` and the `errors` array pointing at the
second entry, and nothing is written.

Or a single entry:

```sh
curl -sS -X POST http://localhost:8000/api/settings/validate \
  -H 'Content-Type: application/json' \
  -d '{"provider": "duckdns", "domain": "sub.duckdns.org", "token": "00000000-0000-0000-0000-000000000000"}'
```

Which answers `200` with `{"valid":true,"warnings":[],"errors":[],"recordCount":...}`.

### List the providers

```sh
curl -sS http://localhost:8000/api/providers
```

### Force a reload

Re-read the settings file without changing it, for example after editing it by
hand:

```sh
curl -sS -X POST http://localhost:8000/api/reload
```

### Poll the records

```sh
curl -sS http://localhost:8000/api/records
```

### Run an update cycle now

```sh
curl -sS http://localhost:8000/update
```

## Settings file format

The file written and read by this API is the same `config.json` you can edit by
hand, at `CONFIG_FILEPATH` (`/updater/data/config.json` in the container). Its
top level holds a `settings` array, and optionally the `_env_seed` key:

```json
{
  "_env_seed": "fe6ea7f9c2348bec9bfc33e192f457f2d0a75d2ef58d3c75808c3a0157034341",
  "settings": [
    {
      "provider": "duckdns",
      "domain": "sub.duckdns.org",
      "token": "00000000-0000-0000-0000-000000000000",
      "ip_version": "ipv4"
    }
  ]
}
```

- `settings` holds one object per DNS record to keep up to date. The `provider`
  key selects the DNS provider, and the remaining keys are the ones documented in
  your provider's page.
- `_env_seed` is a **reserved key** holding the hex SHA-256 of the `CONFIG`
  environment variable value, at the time the file was last synchronized from it.
  It is what makes the settings file, and therefore your web UI edits, survive a
  restart while `CONFIG` is set. See
  [Configuration precedence](../README.md#configuration-precedence-and-restart-safety).
  Do not hand-edit it: it is written and maintained by the program, and any value
  you put there makes the program believe `CONFIG` changed, so `CONFIG` wins
  again and overwrites your file. The value above is the SHA-256 of the exact
  single line you would put in the `CONFIG` environment variable, for example
  `{"settings":[{"provider":"duckdns","domain":"sub.duckdns.org","token":"00000000-0000-0000-0000-000000000000","ip_version":"ipv4"}]}`.
- Any other top level key is preserved when the program rewrites the file, so a
  custom key you add survives an edit made through the UI. The API ignores it.
- The program writes the file atomically (a temporary file renamed over it), with
  a two space indentation and a trailing newline. The order of the top level keys
  is not significant.

The API never returns `_env_seed`: `GET /api/settings` and the raw editor of the
settings page show the `settings` array only.

## Hot reload semantics

The updater keeps the DNS providers and the records in memory, built from the
settings at start up. Every mutating endpoint of this API rebuilds them from the
file it just wrote, without restarting anything:

- A **successful** write is written to the file first, then applied to the running
  updater, so the `200`/`201` response means the change is already live. The
  `records` field of the response is the number of records running after the
  change.
- A **rejected** entry is rejected **before** anything is written: if it is not a
  JSON object, if it produces no DNS provider (missing or unknown `provider`) or
  if the provider itself rejects one of its settings, the settings file is left
  untouched and the answer is a `422` carrying one `errors` entry per problem.
  For `PUT /api/settings`, `POST /api/settings/validate` and the whole document
  at once, every offending entry is reported.
- A change that was written but whose records **fail to build** is **rolled
  back**: the file is restored to the exact entries it had before the change, the
  running records are left as they were, a notification is sent about the
  rollback, and the answer is a `500`. This is rare, since the entries have
  already been validated, and happens for example when the history of a record
  cannot be read from `updates.json`.
- An **update already in flight** for a record that was just reconfigured is
  **discarded** rather than applied. The record identity (domain, provider, IP
  version, IPv6 suffix and proxied flag) is compared under the same lock when the
  result is written back, and a mismatch, or an index that no longer exists, is
  dropped. A stale result therefore never overwrites a record that now points
  somewhere else, and it is not even announced as a notification. The newly
  configured record is updated on the next cycle instead. This is why a record can
  show the `updating` status for a moment after a change and then settle on the
  new configuration.
- The IP address history of a record is restored from `updates.json` when it is
  rebuilt, so reloading does not lose the last known address, and a reload does
  not by itself trigger an update. Records start at the `unset` status and are
  picked up on the next update cycle, or immediately with `GET /update`.
- `POST /api/reload` does the same rebuild from the file, without writing. It
  answers `500` if the file holds settings that cannot be built, and the running
  records are then left untouched.
