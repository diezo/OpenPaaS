# OpenPaaS

Turn any spare Linux VM into your own personal PaaS: automatic subdomains, nginx config, TLS and deployments, all from one domain you own.

> Status: early (v0). Setup, the deploy API and TLS for the root and `deploy.` domains work today. App deployments are next.

## Requirements
- A Debian or Ubuntu VM (amd64 or arm64) with a public IPv4 address
- Ports 80 and 443 open in your cloud provider's firewall
- A domain you control
- A GitHub **classic** personal access token with the `read:packages` scope ([create one](https://github.com/settings/tokens/new?scopes=read:packages&description=OpenPaaS))

## Install
```sh
curl -fsSL https://raw.githubusercontent.com/openpaas/openpaas/main/install.sh | sudo bash
```
This installs nginx, certbot (with the nginx plugin) and Docker, then puts `op` at `/usr/local/bin/op`. Running it again upgrades OpenPaaS.

## Set up
```sh
sudo op setup
```
Setup asks for your GitHub token, your domain and an optional email. It then shows the two DNS records to add:

| Type | Name            | Value       |
|------|-----------------|-------------|
| A    | `example.com`   | your VM IP  |
| A    | `*.example.com` | your VM IP  |

Once DNS checks out, setup configures nginx, starts the deploy API at `https://deploy.example.com` (it restarts automatically and starts on boot), and gets a Let's Encrypt certificate.

To change any value later, run `sudo op setup` again. Pressing Enter keeps the current value.

## Commands
| Command          | Description                         |
|------------------|-------------------------------------|
| `op version`     | Print the version (no setup needed) |
| `sudo op setup`  | Set up or reconfigure this VM       |
| `sudo op status` | Domain, API health, cert expiry     |

## Files
| Path | Contents |
|------|----------|
| `/etc/openpaas/config.json` | Config, including the token (root-only, 0600) |
| `/var/log/openpaas/` | Install and setup logs |
| `/etc/nginx/sites-available/openpaas-*.conf` | Managed nginx sites |
| `/etc/systemd/system/openpaas-api.service` | Deploy API service |

## Development
```sh
go test ./...
GOOS=linux GOARCH=amd64 go build -o dist/op ./cmd/op
# on a test VM:
sudo OP_BINARY=./op bash install.sh
```
To release, push a tag such as `v0.1.0`. GitHub Actions and GoReleaser then publish `op_linux_{amd64,arm64}.tar.gz` and `checksums.txt`, which the installer downloads.
