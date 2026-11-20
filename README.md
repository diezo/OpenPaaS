# OpenPaaS

Turn any spare Linux VM into your own personal PaaS: automatic subdomains, nginx config, TLS and deployments, all from one domain you own.

> Status: early (v0). Setup, the deploy API, and deploying Docker images to subdomains work today. Auto-redeploy on image push is next (see [docs/ROADMAP.md](docs/ROADMAP.md)).

## Requirements
- A Debian or Ubuntu VM (amd64 or arm64) with a public IPv4 address
- Ports 80 and 443 open in your cloud provider's firewall
- A domain you control
- A GitHub **classic** personal access token with the `read:packages` scope ([create one](https://github.com/settings/tokens/new?scopes=read:packages&description=OpenPaaS))

## Install
```sh
curl -fsSL https://raw.githubusercontent.com/diezo/openpaas/main/install.sh | sudo bash
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

## Deploy a service
```sh
sudo op create blog
```
`op create` asks for a Docker image and the port your app listens on inside the container. Use the `:latest` tag so future auto-redeploys pick up new pushes. Images on `ghcr.io` are pulled with your GitHub token.

It then:
1. pulls the image;
2. reserves a free host port (10000–19999, bound to `127.0.0.1` only);
3. starts the container, which restarts on crash and on boot;
4. routes `blog.example.com` to it through nginx;
5. gets a TLS certificate.

If any step before TLS fails, everything is rolled back.

To skip the questions, pass the image and port as flags:
```sh
sudo op create blog --image ghcr.io/you/blog:latest --port 3000
```

## Commands
| Command                  | Description                                     |
|--------------------------|-------------------------------------------------|
| `op version`             | Print the version (no setup needed)             |
| `sudo op setup`          | Set up or reconfigure this VM                   |
| `sudo op status`         | Domain, API health, cert expiry                 |
| `sudo op create <slug>`  | Deploy an image at `https://<slug>.<domain>`    |
| `sudo op list`           | Services with URL, image, ports and live status |
| `sudo op delete <slug>`  | Remove a service's container, route and cert    |

## Files
| Path | Contents |
|------|----------|
| `/etc/openpaas/config.json` | Config, including the token (root-only, 0600) |
| `/etc/openpaas/services.json` | Deployed services and their reserved ports |
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
