# Mangle VPN

Mangle VPN is a self-hosted OpenVPN server with a web interface. People sign
in, add their own devices, and connect with OpenVPN Connect. Administrators
decide who can connect, what each group of people can reach, and see who is
connected right now.

It is a single binary. It runs the web interface, manages OpenVPN and the
firewall, and keeps everything in one SQLite database, so a server needs only
OpenVPN and iptables beside it. Install it on a Linux server, or run it as a
Docker container.

## What it does

**For the people who connect**

- Sign in with a password and an authenticator app, or through Google or
  your identity provider.
- Add a device and open it straight in OpenVPN Connect, or download its
  `.ovpn` profile. Connect can also sign in and add a device by itself.
- See each device's status and how much data it has used, and revoke one
  that is lost or retired.

**For administrators**

- **Users and groups.** Invite people by email, organize them into groups,
  and set per group how many devices each person may add and whether
  two-factor is required.
- **Firewall rules per group.** Each group gets its own rules, enforced with
  iptables as people connect, so contractors can reach staging while
  engineers reach everything.
- **Routes and DNS** for the whole server or per group, a choice between
  routing only your networks or all traffic, and fixed addresses for
  devices that other systems need to recognize.
- **Live clients.** See who is connected, from where, for how long and how
  much data they've moved, and disconnect anyone.
- **Activity.** A searchable audit log of sign-ins, connections and every
  change an administrator makes.
- **Help desk role.** Staff who can unlock accounts, reset two-factor and
  revoke lost devices, without being able to change settings.
- **Single sign-on** with Google or any OpenID Connect provider (Okta,
  Microsoft Entra ID, Authentik, Keycloak), optionally as the only way in
  and creating accounts on first sign-in.
- **Alerts** by email or webhook (Slack, Teams, Google Chat, Discord and
  others) when OpenVPN stops, a certificate is about to expire, an account
  is locked out, and more.
- **Backups** every night, kept on the server and optionally copied to
  S3-compatible storage.
- **HTTPS from Let's Encrypt**, obtained and renewed automatically.
- **Monitoring** through a health check and Prometheus metrics.

## How it works

Mangle VPN is one binary, `mangle-vpn`, with the web interface built in.
On a server, two systemd services run:

| Service | Runs | What it does |
| --- | --- | --- |
| `mangle-web` | `mangle-vpn web` | the web interface and its API, served over HTTPS, and the background work: email, alerts, backups, the revocation list, clean-up |
| `mangle-vpn` | `openvpn` | the system's OpenVPN server, configured by Mangle VPN |

You start `mangle-web` yourself. Setup starts `mangle-vpn` once the VPN is
configured. Around OpenVPN, the binary also runs as short-lived hooks:

- systemd runs it as OpenVPN starts and stops, to write OpenVPN's
  configuration and to open and close its port;
- OpenVPN runs it when a device connects, to decide whether to let it in
  and apply its group's firewall rules;
- OpenVPN runs it again when a device disconnects, to record the
  connection.

Mangle VPN runs its own certificate authority. Every device gets its own
certificate, so revoking a device stops it at once without affecting anyone
else. When a device connects, OpenVPN asks Mangle VPN whether to let it in.
If the answer is yes, the device's address is placed in its group's firewall
chain.

Everything lives under one installation directory, conventionally
`/opt/mangle-vpn`. The binary sits at the top. Beside it, `data/` holds the
database, keys, logs, backups and generated systemd units.

## Requirements

- A Linux server with systemd, on amd64 or arm64, dedicated to Mangle VPN:
  it takes over the firewall.
- OpenVPN 2.6 or newer, which Debian 12 and Ubuntu 24.04 ship. 2.5 also
  works, without kernel acceleration.
- iptables.
- An accurate clock (`systemd-timesyncd` or `chronyd`), since two-factor
  codes are time based.
- Ports open to the internet for the web interface (443, and 80 for
  redirects and Let's Encrypt) and for the VPN (UDP 1194 unless you choose
  otherwise).

Devices need OpenVPN Connect on macOS, Windows, Linux, iOS or Android, or any
OpenVPN 2.4 or newer client.

## Installing

Every step below is ordinary system administration, written out so you can
see what it does and adapt it to how you manage machines. The binary only
sets up what belongs to it.

**1. Build the binary.** On any machine with Go and Node 20 or newer:

```bash
make dist        # dist/mangle-vpn-linux-amd64
make release     # amd64 and arm64
```

**2. Install the prerequisites** on the server:

```bash
# Debian / Ubuntu
apt-get install -y openvpn iptables iptables-persistent ca-certificates

# Fedora / RHEL
dnf install -y openvpn iptables iptables-services
```

On Fedora and RHEL, disable `firewalld`: it contends with these rules over
the same tables.

Optionally, install the data channel offload (DCO) kernel module, which lets
the kernel encrypt VPN traffic instead of OpenVPN, for much higher speeds at
lower CPU. It needs OpenVPN 2.6 or newer:

```bash
apt-get install -y openvpn-dco-dkms       # Debian 12, Ubuntu 24.04
```

Mangle VPN loads the module when OpenVPN starts, and Settings › OpenVPN
shows whether it is in use. Without it, OpenVPN works as usual.
**3. Enable IPv4 forwarding**, so the server can route devices' traffic:

```bash
echo "net.ipv4.ip_forward=1" > /etc/sysctl.d/99-mangle-vpn.conf
sysctl -p /etc/sysctl.d/99-mangle-vpn.conf
```

**4. Set a baseline firewall.** Deny everything inbound except SSH, loopback
and established connections. Mangle VPN opens its own web and VPN ports when
its services start, and closes them when they stop.

```bash
iptables -F
iptables -A INPUT -m conntrack --ctstate ESTABLISHED -j ACCEPT
iptables -A INPUT -p tcp --dport 22 -j ACCEPT
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT
iptables -P INPUT DROP
iptables -P FORWARD ACCEPT
iptables -P OUTPUT ACCEPT

iptables-save > /etc/iptables/rules.v4   # Debian / Ubuntu
service iptables save                    # Fedora / RHEL
```

**5. Copy the binary into place:**

```bash
scp dist/mangle-vpn-linux-amd64 root@server:/tmp/
ssh root@server install -D -m 0755 /tmp/mangle-vpn-linux-amd64 /opt/mangle-vpn/mangle-vpn
```

**6. Prepare the installation.** This creates the database, a self-signed
web certificate and the systemd units, and prints a setup link:

```bash
/opt/mangle-vpn/mangle-vpn install
```

**7. Start the services:**

```bash
systemctl enable --now /opt/mangle-vpn/data/systemd/mangle-web.service
systemctl enable /opt/mangle-vpn/data/systemd/mangle-vpn.service
```

The VPN service is only enabled here. Setup starts it once it has been
configured.

**8. Finish setup in the browser.** Open the link `install` printed. The
browser warns about the self-signed certificate once. The setup code is
in the link; if you open the page another way, `install` also printed the
code on its own. Setup has three steps:

1. **Your organization:** its name, and the address people use to reach
   this page.
2. **The VPN:** the public address devices connect to, the port and
   protocol, and whether devices reach only the networks you list or send
   all their traffic through the VPN. Under *Advanced* you can change the
   addresses devices are given and the certificate key type (ECDSA P-256
   by default, or RSA).
3. **Your administrator account.**

Finishing creates the certificate authority, named after your organization,
and starts OpenVPN. Then you set up two-factor for your own account.

### After setup

- **Set up email** in Settings › Email, so invitations and password resets
  can be sent. Without it, the links are shown to you to pass on.
- **Get a trusted certificate** in Settings › General, by turning on Let's
  Encrypt (the web address must be a DNS name pointing at the server, with
  port 80 or 443 reachable) or by pasting your own.
- **Connect a device yourself** from My devices, then **invite people** from
  Users.
- **Set up offsite backups** in Settings › Backups.

## Running in Docker

The image holds Mangle VPN, OpenVPN 2.6 and iptables. Mangle VPN runs
OpenVPN itself, so there is no systemd inside. Its firewall rules apply
only to the container's own network, so unlike a server install it can
share a machine with other things.

With the `compose.yaml` in this repository:

```bash
docker compose up -d
docker compose logs mangle-vpn | grep setup_code
```

Then open `https://<host>/install?token=<code>` and follow the setup
wizard. When it asks for the VPN's public address, give the address of the
Docker host, since that is where the published port is.

The container needs:

- **`NET_ADMIN` and `/dev/net/tun`**, for OpenVPN's tunnel, routes and
  firewall. It cannot run rootless, or on platforms that forbid these.
- **IPv4 forwarding**, set with `sysctls: net.ipv4.ip_forward=1`. Without
  it, devices connect but reach nothing; the log warns about it.
- **Ports 80, 443 and 1194/udp** published. If you change the VPN's port
  or protocol in setup or in Settings › OpenVPN, publish that one instead.
- **A volume for `/opt/mangle-vpn/data`**, which holds everything worth
  keeping.

Without Compose:

```bash
docker run -d --name mangle-vpn --restart unless-stopped \
  --cap-add NET_ADMIN --device /dev/net/tun \
  --sysctl net.ipv4.ip_forward=1 \
  -p 80:80 -p 443:443 -p 1194:1194/udp \
  -v mangle-data:/opt/mangle-vpn/data \
  jeffmvr/mangle-vpn
```

The image is on Docker Hub as `jeffmvr/mangle-vpn`, for amd64 and arm64.
To build it from this repository instead, run `make docker` or
`docker compose build`.

Devices reach the networks around the Docker host through Docker's own
NAT, so to those networks their traffic comes from the host's address.
Kernel acceleration works when the host has the DCO module loaded; the
container can't load it itself.

To restore a backup, stop the container and run the restore in a
throwaway one with the same volume:

```bash
docker compose stop
docker compose run --rm mangle-vpn restore /opt/mangle-vpn/data/backups/mangle-20261005-030000.tar.gz
docker compose start
```

## Connecting devices

Each person adds their devices from **My devices**. There are three ways to
get a profile into OpenVPN Connect:

- **Open in OpenVPN Connect**, offered when adding a device: the app opens
  and imports the profile itself. The link works once, within five minutes.
- **Import Profile › URL** in the app: enter the server's address and sign
  in. Each import adds a device.
- **Download the `.ovpn` file** and import it.

When connecting, the username is the person's email address and the password
is the current code from their authenticator app. Settings › OpenVPN can ask
for the account password as well.

## Administration

### Groups and firewall rules

Every user belongs to one group. A group sets:

- how many devices each member may add;
- whether two-factor is required;
- extra routes and DNS servers pushed to its members;
- whether unused devices are retired after a time.

Its firewall rules allow or deny destinations by network, port and protocol.
Changes to a group's rules apply to connected members straight away.

### Roles

- **Administrators** can change everything.
- **Help desk** staff see the administration pages read-only. They can help
  members back in: unlock an account, reset two-factor, send a password link,
  disconnect a client, revoke a device. They cannot do this to administrators
  or other help desk staff, and cannot change users, groups, rules or
  settings.
- **Members** manage only their own devices.

### Single sign-on

Settings › Single sign-on works with Google or any OpenID Connect provider.
Register the redirect URI the page shows with your provider. Accounts are
matched by verified email address. For providers that don't say whether an
address is verified, such as Microsoft Entra ID, set *Only accounts from this
domain*.

With *Single sign-on only* turned on, nobody but administrators can use a
password. Administrators keep theirs, so a broken provider can't lock everyone
out. *Create accounts on first sign-in* lets anyone from the allowed domain
in, in a group you choose.

### Security

Settings › Security sets:

- how many wrong passwords or codes lock an account, and for how long;
- how long a session may sit idle and last in all;
- the password rules;
- which networks the administration pages may be used from.

### Certificates

Settings › General lists the certificate authority, the OpenVPN server and
the web server certificates with their expiry dates. Administrators are
warned 30 days before any of them expires.

- The OpenVPN server certificate can be **renewed** there. Devices keep
  working, and it takes effect when OpenVPN restarts.
- The web certificate is replaced by Let's Encrypt or by pasting a new one,
  with no restart.

### Alerts

Settings › Alerts sends email, a webhook message, or both. These are on
unless you switch them off:

- OpenVPN stops when nobody asked it to.
- A certificate is near expiry.
- An account is locked out.
- A nightly backup fails.

You can also turn on alerts for:

- new devices;
- staff signing in;
- sign-ins from new addresses;
- a daily summary.

The webhook body works with Slack, Microsoft Teams, Google Chat, Mattermost,
Rocket.Chat and Discord incoming webhooks.

### Backups and restoring

Every night a backup is written to `data/backups`, keeping seven unless
Settings › Backups says otherwise. The same page:

- takes a backup on demand;
- lists backups for download;
- can copy each one to an S3-compatible bucket (Amazon S3, Backblaze B2,
  Cloudflare R2, Wasabi, MinIO).

A backup on the disk it protects is no help when the disk fails, so set up
the offsite copy.

From the command line:

```bash
mangle-vpn backup                       # into data/backups
mangle-vpn backup -o /mnt/mangle.tar.gz
```

To restore, onto the same server or a new one that has had `mangle-vpn
install` run:

```bash
systemctl stop mangle-web mangle-vpn
mangle-vpn restore mangle-20261005-030000.tar.gz
systemctl start mangle-web mangle-vpn
```

The backup is checked before anything is touched. What it replaces is moved
to `data/pre-restore-<time>` rather than deleted.

A backup holds the database and `data/keys/secret.key`. Private keys,
two-factor secrets and other credentials are encrypted in the database with
that key, so a copy of the database alone gives none of them away. **Keep
backups as safe as the server.**

### Monitoring

- `GET /healthz` answers `200` while the application can serve, and `503`
  when it can't. It needs no sign-in.
- Add `-metrics-listen 127.0.0.1:9100` to `ExecStart` in
  `mangle-web.service` to serve Prometheus metrics on their own listener.
  Besides the Go runtime metrics, it reports:
  - users, locked users and devices;
  - connected clients;
  - whether OpenVPN is up;
  - each certificate's expiry.

  The metrics have no sign-in, so bind them to localhost or a private
  network.
- Logs are under Logs in the interface, and in `data/logs` on disk. They
  rotate and are compressed automatically.

### Serving and reverse proxies

The web service serves HTTPS on the ports in Settings › General, and
redirects plain HTTP to it. These flags on `mangle-vpn web` override that:

| Flag | Effect |
| --- | --- |
| `-listen` | the HTTPS address, or a unix socket path |
| `-listen-http` | the redirect listener's address, or `off` |
| `-trust-proxy` | believe `X-Forwarded-For` and `X-Forwarded-Proto` |
| `-insecure` | plain HTTP with no TLS, for development only |

No reverse proxy is needed. If you put one in front, use `-trust-proxy` so
the audit log records people's real addresses. Leave it off otherwise:
without a proxy, those headers are whatever the caller says.

### Performance

- **Install the DCO kernel module** (see *Installing*). It is by far the
  biggest gain. Settings › OpenVPN shows *Kernel acceleration: On* when it
  is working. A few options can stop OpenVPN from using it: if it shows
  *Off* with the module installed, the OpenVPN log says why.
- **Use UDP** unless a network forces TCP. TCP inside TCP slows down badly
  on lossy connections.
- **Choose a server with AES instructions** (any recent x86 or ARM server).
  Traffic is encrypted with AES-GCM, or ChaCha20 for devices without them.
- **For fast connections over long distances**, give sockets larger
  buffers. OpenVPN's UDP socket uses the kernel's default size, which is
  small for a link with a lot of data in flight. On a server dedicated to
  Mangle VPN:

  ```bash
  cat > /etc/sysctl.d/99-mangle-vpn-buffers.conf <<'CONF'
  net.core.rmem_default=1048576
  net.core.wmem_default=1048576
  net.core.rmem_max=4194304
  net.core.wmem_max=4194304
  CONF
  sysctl --system
  systemctl restart mangle-vpn
  ```

### Sharing port 443

On networks that allow only web traffic, Settings › OpenVPN can run the VPN
on TCP 443 and pass web traffic through to the web interface. To use it,
first move the web interface's HTTPS port to another port, such as 8443.

## Commands

| Command | |
| --- | --- |
| `mangle-vpn install` | prepare a new installation, or regenerate the systemd units of an existing one |
| `mangle-vpn web` | run the web interface and background work (the `mangle-web` service) |
| `mangle-vpn run` | run everything, OpenVPN included, in one process: the Docker image's command |
| `mangle-vpn health` | check that the web interface answers, for the image's health check |
| `mangle-vpn backup [-o file]` | write a backup |
| `mangle-vpn restore <file>` | restore a backup, with the services stopped |
| `mangle-vpn version` | print the version |

Every command takes `-root` to point at an installation other than the one
the binary sits in.

## Upgrading

Build the new binary, replace the old one, and restart the services:

```bash
install -m 0755 /tmp/mangle-vpn-linux-amd64 /opt/mangle-vpn/mangle-vpn
systemctl restart mangle-web
```

In Docker, rebuild or pull the image and recreate the container:

```bash
docker compose up -d --build
```

The database is upgraded automatically when the new version starts. When a
release changes the systemd units, run `mangle-vpn install` again after
replacing the binary: it rewrites the units of an existing installation, and
removes any that are no longer used, such as the old `mangle-tasks` service.
Restarting `mangle-vpn` as well picks up any change to how OpenVPN is
configured, but it disconnects everyone briefly, so choose a quiet moment.

## Development

```bash
make dev     # serve at http://localhost:9443, rebuilding on every change
make test    # run the tests
make dist    # build the Linux binary for a server
```

The first `make dev` prepares a development installation in `../data` and
prints a setup code for the wizard. `ui/README.md` covers working on the web
interface.

## License

Mangle VPN is free software, licensed under the [GNU General Public License
v3.0](LICENSE). You may use, study, change and share it, including running it
for your organization. If you distribute it, changed or not, you must do so
under the same license and make the source available.
