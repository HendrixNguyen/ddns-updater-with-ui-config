# Lightweight universal DDNS Updater program

Program to keep DNS A and/or AAAA records updated for multiple DNS providers

<img height="200" alt="DDNS Updater logo" src="https://raw.githubusercontent.com/qdm12/ddns-updater/master/readme/ddnsgopher.svg">

[![Build status](https://github.com/qdm12/ddns-updater/actions/workflows/build.yml/badge.svg)](https://github.com/qdm12/ddns-updater/actions/workflows/build.yml)

![Last release](https://img.shields.io/github/release/qdm12/ddns-updater?label=Last%20release)
![Last Docker tag](https://img.shields.io/github/v/release/qdm12/ddns-updater?sort=semver&label=Last%20Docker%20tag)
[![Last release size](https://img.shields.io/docker/image-size/qmcgaw/ddns-updater?sort=semver&label=Last%20released%20image)](https://hub.docker.com/r/qmcgaw/ddns-updater/tags?page=1&ordering=last_updated)
![GitHub last release date](https://img.shields.io/github/release-date/qdm12/ddns-updater?label=Last%20release%20date)
![Commits since release](https://img.shields.io/github/commits-since/qdm12/ddns-updater/latest?sort=semver)

[![Latest size](https://img.shields.io/docker/image-size/qmcgaw/ddns-updater/latest?label=Latest%20image)](https://hub.docker.com/r/qmcgaw/ddns-updater/tags)

[![GitHub last commit](https://img.shields.io/github/last-commit/qdm12/ddns-updater.svg)](https://github.com/qdm12/ddns-updater/commits/main)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/y/qdm12/ddns-updater.svg)](https://github.com/qdm12/ddns-updater/graphs/contributors)
[![GitHub closed PRs](https://img.shields.io/github/issues-pr-closed/qdm12/ddns-updater.svg)](https://github.com/qdm12/ddns-updater/pulls?q=is%3Apr+is%3Aclosed)
[![GitHub issues](https://img.shields.io/github/issues/qdm12/ddns-updater.svg)](https://github.com/qdm12/ddns-updater/issues)
[![GitHub closed issues](https://img.shields.io/github/issues-closed/qdm12/ddns-updater.svg)](https://github.com/qdm12/ddns-updater/issues?q=is%3Aissue+is%3Aclosed)

![Code size](https://img.shields.io/github/languages/code-size/qdm12/ddns-updater)
![GitHub repo size](https://img.shields.io/github/repo-size/qdm12/ddns-updater)
![Go version](https://img.shields.io/github/go-mod/go-version/qdm12/ddns-updater)

[![MIT](https://img.shields.io/github/license/qdm12/ddns-updater)](LICENSE)
![Visitors count](https://visitor-badge.laobi.icu/badge?page_id=ddns-updater.readme)

## Versioned documentation

This readme and the [docs/](docs/) directory are **versioned** to match the program version:

| Version | Readme link | Docs link |
| --- | --- | --- |
| Latest | [README](https://github.com/qdm12/ddns-updater/blob/master/README.md) | [docs/](https://github.com/qdm12/ddns-updater/tree/master/docs) |
| `v2.8` | [README](https://github.com/qdm12/ddns-updater/blob/v2.8.0/README.md) | [docs/](https://github.com/qdm12/ddns-updater/blob/v2.8.0/docs) |
| `v2.7` | [README](https://github.com/qdm12/ddns-updater/blob/v2.7.1/README.md) | [docs/](https://github.com/qdm12/ddns-updater/blob/v2.7.1/docs) |
| `v2.6` | [README](https://github.com/qdm12/ddns-updater/blob/v2.6.1/README.md) | [docs/](https://github.com/qdm12/ddns-updater/blob/v2.6.1/docs) |
| `v2.5` | [README](https://github.com/qdm12/ddns-updater/blob/v2.5.0/README.md) | [docs/](https://github.com/qdm12/ddns-updater/blob/v2.5.0/docs) |

## Features

- Available as a Docker image [`ghcr.io/qdm12/ddns-updater`]((https://github.com/qdm12/ddns-updater/pkgs/container/ddns-updater)) and [`qmcgaw/ddns-updater`](https://hub.docker.com/r/qmcgaw/ddns-updater)
- Available as [zero-dependency binaries for Linux, Windows and MacOS](https://github.com/qdm12/ddns-updater/releases)
- 🆕 Available in the AUR as [`ddns-updater`](https://aur.archlinux.org/packages/ddns-updater) - see [#808](https://github.com/qdm12/ddns-updater/discussions/808)
- Updates periodically A records for different DNS providers:
  - Aliyun
  - AllInkl
  - Bunny
  - ChangeIP
  - Cloudflare
  - DD24
  - DDNSS.de
  - deSEC
  - DigitalOcean
  - Domeneshop
  - DonDominio
  - DNSOMatic
  - DNSPod
  - Dreamhost
  - DuckDNS
  - DynDNS
  - Dynu
  - DynV6
  - EasyDNS
  - FreeDNS
  - Gandi
  - GCP
  - Gigahost.no
  - GoDaddy
  - GoIP.de
  - He.net
  - Hetzner (legacy API)
  - Hetzner Cloud
  - Hostinger
  - Infomaniak
  - INWX
    - Ionos
    - ipv64
  - Linode
  - Loopia
  - LuaDNS
  - Myaddr
  - Name.com
  - Namecheap
  - NameSilo
  - Netcup
  - NoIP
  - Now-DNS
  - Njalla
  - OpenDNS
  - OVH
  - Porkbun
  - Route53
  - Scaleway
  - Selfhost.de
  - Servercow.de
  - Spaceship
  - Spdyn
  - Strato.de
  - Variomedia.de
  - Vultr
  - Zoneedit
  - **Want more?** [Create an issue for it](https://github.com/qdm12/ddns-updater/issues/new/choose)!
- Web user interface (Desktop)

    ![Web UI](readme/webui-desktop.gif)

- Web user interface (Mobile)

    ![Mobile Web UI](readme/webui-mobile.png)

- 🆕 [Web configuration page](#web-ui-and-rest-api) to add, edit, duplicate and delete records from your browser, saved to `config.json` and applied without restarting
- 🆕 [JSON REST API](#web-ui-and-rest-api) to read the records and manage the settings programmatically
- Send notifications with [**Shoutrrr**](https://containrrr.dev/shoutrrr/v0.8/services/overview/) using `SHOUTRRR_ADDRESSES`
- Container (Docker/K8s) specific features:
  - Lightweight 12MB Docker image based on the Scratch Docker image
  - Docker healthcheck verifying the DNS resolution of your domains
  - Images compatible with `amd64`, `386`, `arm64`, `armv7`, `armv6`, `s390x`, `ppc64le`, `riscv64` CPU architectures
- Persistence with a JSON file *updates.json* to store old IP addresses with change times for each record

## Setup

### Binary programs

1. Download the pre-built program for your platform from the assets of a release in the [releases page](https://github.com/qdm12/ddns-updater/releases). You can alternatively download, build and install the latest version of the program by installing [Go](https://golang.org/doc/install) and then run `go install github.com/qdm12/ddns-updater/cmd/ddns-updater@latest`.
1. For Linux and MacOS, make the program executable with `chmod +x ddns-updater`.
1. In the directory where the program is saved, create a directory `data`.
1. Write a JSON configuration in `data/config.json`, for example:

    ```json
    {
        "settings": [
            {
                "provider": "namecheap",
                "domain": "sub.example.com",
                "password": "e5322165c1d74692bfa6d807100c0310"
            }
        ]
    }
    ```

    You can find more information in the [configuration section](#configuration) to customize it.
1. Run the program with `./ddns-updater` (`./ddns-updater.exe` on Windows) or by double-clicking on it.
1. The following is **optional**.
    - You can customize the program behavior using either [environment variables](#environment-variables) or flags. For flags, there is a flag corresponding to each environment variable, where it's all lowercase and underscores are replaced with dashes. For example the environment variable `LOG_LEVEL` translates into `--log-level`.

### Container

[➡️ Qnap guide by @Araminta](https://github.com/qdm12/ddns-updater/issues/708)

1. Create a directory, for example, *data* which is:
    - owned by user id `1000`, which is the built-in user ID of the ddns-updater container
    - has user read+write+execute permissions

    ```sh
    mkdir data
    chown 1000 data
    chmod u+r+w+x data
    ```

    If you want to use another user ID, [build the image yourself](#build-the-image) with `--build-arg UID=<your-uid>`. You could also just run the container as root with `--user="0"` but this is not advised security wise.

1. Similarly, create a *data/config.json* file which is:
    - owned by user id `1000`
    - has user read permissions

    ```sh
    touch data/config.json
    chmod u+r data/config.json
    ```

1. Edit *data/config.json*, for example:

    ```json
    {
        "settings": [
            {
                "provider": "namecheap",
                "domain": "sub.example.com",
                "password": "e5322165c1d74692bfa6d807100c0310"
            }
        ]
    }
    ```

    You can find more information in the [configuration section](#configuration) to customize it.

1. Run the container with

    ```sh
    docker run -d -p 8000:8000/tcp -v "$(pwd)"/data:/updater/data ghcr.io/qdm12/ddns-updater
    ```

1. The following is **optional**.
    - You can customize the program behavior using [environment variables](#environment-variables)
    - You can use [docker-compose.yml](docker-compose.yml) with `docker-compose up -d`
    - **Kubernetes**: check out the [k8s directory](k8s) for an installation guide and examples.
    - Other [Docker image tags are available](https://github.com/qdm12/ddns-updater/pkgs/container/ddns-updater)
    - You can update the image with `docker pull ghcr.io/qdm12/ddns-updater`
    - You can set your JSON configuration as a single environment variable line (i.e. `{"settings": [{"provider": "namecheap", ...}]}`), which takes precedence over config.json. Note however that if you don't bind mount the `/updater/data` directory, there won't be a persistent database file `/updater/updates.json` but it will still work.

### Home Assistant

To install ddns-updater on [Home Assistant](https://home-assistant.io/), follow the installation instructions on [ha-ddns-updater](https://github.com/ha-ddns-updater/ha-ddns-updater/).

## Configuration

Start by having the following content in *config.json*, or in your `CONFIG` environment variable:

```json
{
    "settings": [
        {
            "provider": "",
        },
        {
            "provider": "",
        }
    ]
}
```

For each setting, you need to fill in parameters.
Check the documentation for your DNS provider:

- [Aliyun](docs/aliyun.md)
- [Allinkl](docs/allinkl.md)
- [Bunny](docs/bunny.md)
- [ChangeIP](docs/changeip.md)
- [Cloudflare](docs/cloudflare.md)
- [DD24](docs/dd24.md)
- [DDNSS.de](docs/ddnss.de.md)
- [deSEC](docs/desec.md)
- [DigitalOcean](docs/digitalocean.md)
- [Domeneshop](docs/domeneshop.md)
- [DonDominio](docs/dondominio.md)
- [DNSOMatic](docs/dnsomatic.md)
- [DNSPod](docs/dnspod.md)
- [Dreamhost](docs/dreamhost.md)
- [DuckDNS](docs/duckdns.md)
- [DynDNS](docs/dyndns.md)
- [Dynu](docs/dynu.md)
- [DynV6](docs/dynv6.md)
- [EasyDNS](docs/easydns.md)
- [FreeDNS](docs/freedns.md)
- [Gandi](docs/gandi.md)
- [GCP](docs/gcp.md)
- [Gigahost.no](docs/gigahostno.md)
- [GoDaddy](docs/godaddy.md)
- [GoIP.de](docs/goip.md)
- [He.net](docs/he.net.md)
- [Hetzner](docs/hetzner.md)
- [HetznerCloud](docs/hetznercloud.md)
- [Hostinger](docs/hostinger.md)
- [Infomaniak](docs/infomaniak.md)
- [INWX](docs/inwx.md)
- [Ionos](docs/ionos.md)
- [IPv64](docs/ipv64.md)
- [Linode](docs/linode.md)
- [Loopia](docs/loopia.md)
- [LuaDNS](docs/luadns.md)
- [Myaddr](docs/myaddr.md)
- [Name.com](docs/name.com.md)
- [Namecheap](docs/namecheap.md)
- [NameSilo](docs/namesilo.md)
- [Netcup](docs/netcup.md)
- [NoIP](docs/noip.md)
- [Now-DNS](docs/nowdns.md)
- [Njalla](docs/njalla.md)
- [OpenDNS](docs/opendns.md)
- [OVH](docs/ovh.md)
- [Porkbun](docs/porkbun.md)
- [Route53](docs/route53.md)
- [Scaleway](docs/scaleway.md)
- [Selfhost.de](docs/selfhost.de.md)
- [Servercow.de](docs/servercow.md)
- [Spaceship](docs/spaceship.md)
- [Spdyn](docs/spdyn.md)
- [Strato.de](docs/strato.md)
- [Variomedia.de](docs/variomedia.md)
- [Vultr](docs/vultr.md)
- [Zoneedit](docs/zoneedit.md)
- [Custom](docs/custom.md)

Note that:

- you can specify multiple owners/hosts for the same domain using a comma separated list. For example with `"domain": "example.com,sub.example.com,sub2.example.com",`.
⚠️ this is a bit different for DuckDNS and GoIP, see their respective documentation.

💡 You do not have to hand-write this JSON: the [settings page](#settings-page) lets you add, edit and delete these settings from your browser, and the [REST API](#rest-api) lets you script it.

### Environment variables

🆕 There are now flags equivalent for each variable below, for example `--log-level`.

| Environment variable | Default | Description |
| --- | --- | --- |
| `CONFIG` | | One line JSON object containing the entire config (takes precedence over config.json file) if specified |
| `PERIOD` | `5m` | Default period of IP address check, following [this format](https://golang.org/pkg/time/#ParseDuration) |
| `PUBLICIP_FETCHERS` | `all` | Comma separated fetcher types to obtain the public IP address from `http` and `dns` |
| `PUBLICIP_HTTP_PROVIDERS` | `all` | Comma separated providers to obtain the public IP address (ipv4 or ipv6). See the [Public IP section](#public-ip) |
| `PUBLICIPV4_HTTP_PROVIDERS` | `all` | Comma separated providers to obtain the public IPv4 address only. See the [Public IP section](#public-ip) |
| `PUBLICIPV6_HTTP_PROVIDERS` | `all` | Comma separated providers to obtain the public IPv6 address only. See the [Public IP section](#public-ip) |
| `PUBLICIP_DNS_PROVIDERS` | `all` | Comma separated providers to obtain the public IP address (IPv4 and/or IPv6). See the [Public IP section](#public-ip) |
| `PUBLICIP_DNS_TIMEOUT` | `3s` | Public IP DNS query timeout |
| `UPDATE_COOLDOWN_PERIOD` | `5m` | Duration to cooldown between updates for each record. This is useful to avoid being rate limited or banned. |
| `HTTP_TIMEOUT` | `10s` | Timeout for all HTTP requests |
| `SERVER_ENABLED` | `yes` | Enable the web server and web UI |
| `LISTENING_ADDRESS` | `:8000` | Internal TCP listening port for the web UI |
| `ROOT_URL` | `/` | URL path to append to all paths to the webUI (i.e. `/ddns` for accessing `https://example.com/ddns` through a proxy) |
| `HEALTH_SERVER_ADDRESS` | `127.0.0.1:9999` | Health server listening address |
| `HEALTH_HEALTHCHECKSIO_BASE_URL` | `https://hc-ping.com` | Base URL for the [healthchecks.io](https://healthchecks.io) server |
| `HEALTH_HEALTHCHECKSIO_UUID` | | UUID to idenfity with the [healthchecks.io](https://healthchecks.io) server |
| `DATADIR` | `/updater/data` | Directory to read and write data files from internally |
| `CONFIG_FILEPATH` | `/updater/data/config.json` | Path to the JSON configuration file |
| `BACKUP_PERIOD` | `0` | Set to a period (i.e. `72h15m`) to enable zip backups of data/config.json and data/updates.json in a zip file |
| `BACKUP_DIRECTORY` | `/updater/data` | Directory to write backup zip files to if `BACKUP_PERIOD` is not `0`. |
| `RESOLVER_ADDRESS` | Your network DNS | A plaintext DNS address to use to resolve your domain names defined in your settings only. For example it can be `1.1.1.1:53`. This is useful for split dns, see [#389](https://github.com/qdm12/ddns-updater/issues/389) |
| `LOG_LEVEL` | `info` | Level of logging, `debug`, `info`, `warning` or `error` |
| `LOG_CALLER` | `hidden` | Show caller per log line, `hidden` or `short` |
| `SHOUTRRR_ADDRESSES` | | (optional) Comma separated list of [Shoutrrr addresses](https://containrrr.dev/shoutrrr/v0.8/services/overview/) (notification services) |
| `SHOUTRRR_DEFAULT_TITLE` | `DDNS Updater` | Default title for Shoutrrr notifications |
| `TZ` | | Timezone to have accurate times, i.e. `America/Montreal` |
| `UMASK` | System current umask | Umask to set for the program in octal, i.e. `0022` |

#### Public IP

By default, all public IP fetching types are used and cycled (over DNS and over HTTPs).

On top of that, for each fetching method, all echo services available are cycled on each request.

This allows you not to be blocked for making too many requests.

You can otherwise customize it with the following:

- `PUBLICIP_HTTP_PROVIDERS` gets your public IPv4 or IPv6 address. It can be one or more of the following:
  - `ipify` using [https://api64.ipify.org](https://api64.ipify.org)
  - `ifconfig` using [https://ifconfig.io/ip](https://ifconfig.io/ip)
  - `ipinfo` using [https://ipinfo.io/ip](https://ipinfo.io/ip)
  - `spdyn` using [https://checkip.spdyn.de](https://checkip.spdyn.de/)
  - `ipleak` using [https://ipleak.net/json](https://ipleak.net/json)
  - `icanhazip` using [https://icanhazip.com](https://icanhazip.com)
  - `ident` using [https://ident.me](https://ident.me)
  - `nnev` using [https://ip.nnev.de](https://ip.nnev.de)
  - `wtfismyip` using [https://wtfismyip.com/text](https://wtfismyip.com/text)
  - `seeip` using [https://api.seeip.org](https://api.seeip.org)
  - `changeip` using [https://ip.changeip.com](https://ip.changeip.com)
  - You can also specify an HTTPS URL with prefix `url:` for example `url:https://ipinfo.io/ip`
- `PUBLICIPV4_HTTP_PROVIDERS` gets your public IPv4 address only. It can be one or more of the following:
  - `ipleak` using [https://ipv4.ipleak.net/json](https://ipv4.ipleak.net/json)
  - `ipify` using [https://api.ipify.org](https://api.ipify.org)
  - `icanhazip` using [https://ipv4.icanhazip.com](https://ipv4.icanhazip.com)
  - `ident` using [https://v4.ident.me](https://v4.ident.me)
  - `nnev` using [https://ip4.nnev.de](https://ip4.nnev.de)
  - `wtfismyip` using [https://ipv4.wtfismyip.com/text](https://ipv4.wtfismyip.com/text)
  - `seeip` using [https://ipv4.seeip.org](https://ipv4.seeip.org)
  - You can also specify an HTTPS URL with prefix `url:` for example `url:https://ipinfo.io/ip`
- `PUBLICIPV6_HTTP_PROVIDERS` gets your public IPv6 address only. It can be one or more of the following:
  - `ipleak` using [https://ipv6.ipleak.net/json](https://ipv6.ipleak.net/json)
  - `ipify` using [https://api6.ipify.org](https://api6.ipify.org)
  - `icanhazip` using [https://ipv6.icanhazip.com](https://ipv6.icanhazip.com)
  - `ident` using [https://v6.ident.me](https://v6.ident.me)
  - `nnev` using [https://ip6.nnev.de](https://ip6.nnev.de)
  - `wtfismyip` using [https://ipv6.wtfismyip.com/text](https://ipv6.wtfismyip.com/text)
  - `seeip` using [https://ipv6.seeip.org](https://ipv6.seeip.org)
  - You can also specify an HTTPS URL with prefix `url:` for example `url:https://ipinfo.io/ip`
- `PUBLICIP_DNS_PROVIDERS` gets your public IPv4 address only or IPv6 address only or one of them (see [#136](https://github.com/qdm12/ddns-updater/issues/136)). It can be one or more of the following:
  - `cloudflare`
  - `opendns`

### Host firewall

If you have a host firewall in place, this container needs the following ports:

- TCP 443 outbound for outbound HTTPS
- UDP 53 outbound for outbound DNS resolution
- TCP 8000 inbound (or other) for the WebUI

## Web UI and REST API

The web server is enabled by default (`SERVER_ENABLED=yes`) and listens on `LISTENING_ADDRESS` (`:8000`). All of its paths are served under `ROOT_URL` (`/` by default), so with the defaults the dashboard is at `http://localhost:8000/`.

| Path | Description |
| --- | --- |
| `{ROOT_URL}/` | Records dashboard, refreshed every 10 seconds |
| `{ROOT_URL}/settings` | 🆕 [Settings page](#settings-page), to manage the records |
| `{ROOT_URL}/update` | Force an update cycle of all the records now |
| `{ROOT_URL}/api/...` | 🆕 [JSON REST API](#rest-api) |

### ⚠️ Security warning

**The web UI and the API are unauthenticated.** There is no login, no API key, no token and no permission of any kind. Anyone who can open a TCP connection to `LISTENING_ADDRESS` can:

- read your DNS provider credentials in clear text, from the settings page, from its raw JSON editor and from `GET {ROOT_URL}/api/settings`
- add, change or delete records, and therefore repoint any domain you manage to an address of their choosing
- trigger update cycles at will

We recommend **not exposing the listening port to the public internet**, and more specifically:

- binding it to a private interface or a loopback address, for example `LISTENING_ADDRESS=127.0.0.1:8000`, and reaching it over a VPN or an SSH tunnel
- when it has to be reachable from elsewhere, running it **behind a reverse proxy that adds authentication**, and using `ROOT_URL` to mount the program under a subpath of the proxy

### Settings page

🆕 Your records can now be managed from your browser at `{ROOT_URL}/settings`, without editing any file and **without restarting the container**:

- **Add record** opens a form with a provider picker and the credential fields of the selected provider, generated from the providers the program supports
- **Edit** and **Delete** on each configured record, with a confirmation dialog before a deletion, and **Duplicate** to copy one into a new entry
- a **Filter records** box appears above the list once you have more than 8 records, and matches on any part of an entry
- a **Raw JSON** tab edits the whole `{"settings":[...]}` document, with a **Validate** button that checks it without saving, and a **Save all** button
- entries are validated with the same code path as the program start, so a rejected entry is reported inline and never written
- warnings found in the settings (for example a retro-compatible key) are shown at the top of the page and can be dismissed
- a **Reload** button re-reads the settings file from disk without changing it
- a dark/light theme toggle, remembered per browser

Each change is written to the settings file (`CONFIG_FILEPATH`, `/updater/data/config.json` in the container) atomically, and hot-reloaded into the running updater **immediately**. There is no restart and no lost update cycle: the new records start being updated on the next cycle, and `GET {ROOT_URL}/update` forces one right away. If the new settings cannot be turned into DNS records, the file is rolled back to what it was and nothing changes.

The page also shows the settings file path, the number of running records, and which configuration source is in effect (see below). JavaScript is required; without it the page tells you to edit the settings file directly.

💡 The UI has no build step: it is plain HTML, CSS and JavaScript embedded in the binary. The pages load the [Tailwind](https://tailwindcss.com) browser build from `https://cdn.tailwindcss.com`, so the **browser** rendering the page needs outbound internet access to that CDN for the full styling. The bundled `static/styles.css` is a self-contained fallback for the same design, so the pages remain usable when the CDN is unreachable, for example on an air-gapped network.
⚠️ The pages must be **served by the program**, never opened from a `file://` URL: a browser blocks the API requests of a local file, so the page would stay empty.

### Configuration precedence and restart safety

`config.json` and the `CONFIG` environment variable can both carry the settings. Now that the settings file can be edited at runtime, the program needs to know which one wins at start up, without clobbering what you just edited in the UI. It does so with a seed stored in the file itself, under the reserved `_env_seed` key: the hex SHA-256 of the `CONFIG` value the file was last synchronized from.

The rules, applied at every start up and on every `POST {ROOT_URL}/api/reload`, refine the `CONFIG` row of the [environment variables table](#environment-variables):

- **`CONFIG` is not set**: the settings file is the source of truth, so your UI edits are always used. If the file does not exist, an empty one is created.
- **`CONFIG` is set and the file's `_env_seed` equals the SHA-256 of the `CONFIG` value**: the **file wins**. The environment variable has not changed since the file was last synchronized from it, so the file, including your UI edits, survives the restart.
- **`CONFIG` is set and the seed differs, or the file has no `_env_seed` at all**: the **environment variable wins**. It is written to the file, stamping the new `_env_seed`, and used from there. A file with no seed is a file written before this feature existed, so the first start after upgrading gives `CONFIG` the priority once.

In short: **change `CONFIG` in your compose file to take effect again, leave it alone and the settings file edited from the web UI stays authoritative.**

`_env_seed` is a reserved key: never add it to your own configuration and never hand-edit it. Any value you put there makes the program believe `CONFIG` changed, so `CONFIG` wins again and overwrites your file. Any *other* top level key of the settings file is preserved when the program rewrites it, so a custom key you added survives an edit made through the UI.

The settings page reports the state in effect: the settings file path, the number of running records, a `source: file` / `source: env` / `source: empty` badge and a `CONFIG: set` / `CONFIG: unset` badge, with a banner explaining what happens if you edit one or the other. `GET {ROOT_URL}/api/settings` returns the same information in its `source`, `env` and `filePath` fields.

### REST API

The same web server exposes a JSON REST API, which the web UI itself uses. 🆕 No new environment variable is needed to enable it, it is served whenever the web server is enabled.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `{ROOT_URL}/` | Records dashboard (HTML) |
| `GET` | `{ROOT_URL}/settings` | Settings page (HTML) |
| `GET` | `{ROOT_URL}/update` | Force an update cycle now, `202` on success |
| `GET` | `{ROOT_URL}/static/*` | Static assets of the web UI |
| `GET` | `{ROOT_URL}/api/providers` | Supported providers and the fields each one accepts |
| `GET` | `{ROOT_URL}/api/settings` | Current settings entries and configuration source |
| `POST` | `{ROOT_URL}/api/settings` | Add one settings entry, `201` on success |
| `PUT` | `{ROOT_URL}/api/settings` | Replace all the settings entries |
| `PUT` | `{ROOT_URL}/api/settings/{index}` | Replace the settings entry at `{index}` |
| `DELETE` | `{ROOT_URL}/api/settings/{index}` | Delete the settings entry at `{index}` |
| `POST` | `{ROOT_URL}/api/settings/validate` | Validate settings entries without saving them |
| `POST` | `{ROOT_URL}/api/reload` | Re-read the settings file and apply it, without saving |
| `GET` | `{ROOT_URL}/api/records` | The records and their last known status |

All the mutating endpoints write the settings file and hot reload the running updater, without a restart. Request bodies must be sent as `Content-Type: application/json` and are limited to 1 MiB. Errors are always `{"errors":["..."]}`.

```sh
# add a record
curl -sS -X POST http://localhost:8000/api/settings \
  -H 'Content-Type: application/json' \
  -d '{"provider":"duckdns","domain":"sub.duckdns.org","token":"00000000-0000-0000-0000-000000000000","ip_version":"ipv4"}'

# and change it
curl -sS -X PUT http://localhost:8000/api/settings/0 \
  -H 'Content-Type: application/json' \
  -d '{"provider":"duckdns","domain":"other.duckdns.org","token":"00000000-0000-0000-0000-000000000000","ip_version":"ipv4"}'
```

📖 See **[docs/api.md](docs/api.md)** for the full reference: every endpoint with its request and response shapes, the status codes, copy-pasteable `curl` examples and the hot reload semantics. ⚠️ Reminder: it is unauthenticated, as described above.

## Architecture

At program start and every period (5 minutes by default):

1. Fetch your public IP address
1. For each record:
    1. DNS resolve it to obtain its current IP address(es)
        - If the resolution fails, update the record with your public IP address by calling the DNS provider API and finish
    1. Check if your public IP address is within the resolved IP addresses
        - Yes: skip the update
        - No: update the record with your public IP address by calling the DNS provider API

💡 We do DNS resolution every period so it detects a change made to the record manually, for example on the DNS provider web UI
💡 As DNS resolutions are essentially free and without rate limiting, these are great to avoid getting banned for too many requests.

### Special case: Cloudflare

For Cloudflare records with the `proxied` option, the following is done.

At program start and every period (5 minutes by default), for each record:

1. Fetch your public IP address
1. For each record:
    1. Check the last IP address (persisted in `updates.json`) for that record
        - If it doesn't exist, update the record with your public IP address by calling the DNS provider API and finish
    1. Check if your public IP address matches the last IP address you updated the record with
        - Yes: skip the update
        - No: update the record with your public IP address by calling the DNS provider API

This is the only way as doing a DNS resolution on the record will give the IP address of a Cloudflare server instead of your server.

⚠️ This has the disadvantage that if the record is changed manually, the program will not detect it.
We could do an API call to get the record IP address every period, but that would get you banned especially with a low period duration.

## Testing

- The automated healthcheck verifies all your records are up to date [using DNS lookups](internal/health/check.go#L59). For the duration of the record TTL (when known by the provider) after a successful update, the previous IP address is also accepted since resolvers may not have propagated the update yet.
- You can also manually check, by:
    1. Going to your DNS management webpage
    1. Setting your record to `127.0.0.1`
    1. Run the container
    1. Refresh the DNS management webpage and verify the update happened

## Build the image

You can build the image yourself with:

```sh
docker build -t ghcr.io/qdm12/ddns-updater https://github.com/qdm12/ddns-updater.git
```

You can use optional build arguments with `--build-arg KEY=VALUE` from the table below:

| Build argument | Default | Description |
| --- | --- | --- |
| `UID` | `1000` | User ID running the container |
| `GID` | `1000` | User group ID running the container |
| `VERSION` | `unknown` | Version of the program and Docker image |
| `CREATED` | `an unknown date` | Build date of the program and Docker image |
| `COMMIT` | `unknown` | Commit hash of the program and Docker image |

## Development and contributing

- [Contribute with code](.github/CONTRIBUTING.md)
- [Github workflows to know what's building](https://github.com/qdm12/ddns-updater/actions)
- [List of issues and feature requests](https://github.com/qdm12/ddns-updater/issues)

## License

This repository is under an [MIT license](LICENSE)

## Used in external projects

- [Starttoaster/docker-traefik](https://github.com/Starttoaster/docker-traefik#home-networks-extra-credit-dynamic-dns)

## Support

Sponsor me on [Github](https://github.com/sponsors/qdm12) or donate to [paypal.me/qmcgaw](https://www.paypal.me/qmcgaw)

Many thanks to J. Famiglietti for supporting me financially 🥇👍
