# Tailcat ACAP for Axis Devices

[![Release](https://img.shields.io/github/v/release/Mo3he/Axis_Cam_Tailcat?style=flat)](https://github.com/Mo3he/Axis_Cam_Tailcat/releases)
[![Build](https://github.com/Mo3he/Axis_Cam_Tailcat/actions/workflows/build.yml/badge.svg)](https://github.com/Mo3he/Axis_Cam_Tailcat/actions/workflows/build.yml)
[![License](https://img.shields.io/github/license/Mo3he/Axis_Cam_Tailcat?style=flat)](LICENSE)
[![Super-Linter](https://github.com/Mo3he/Axis_Cam_Tailcat/actions/workflows/super-linter.yml/badge.svg)](https://github.com/Mo3he/Axis_Cam_Tailcat/actions/workflows/super-linter.yml)
[![Sponsor](https://img.shields.io/badge/Sponsor%20My%20Work-EA4AAA?style=flat&logo=github&logoColor=white)](https://github.com/sponsors/Mo3he)
[![Buy Me A Coffee](https://img.shields.io/badge/Buy%20Me%20A%20Coffee-FFDD00?style=flat&logo=buy-me-a-coffee&logoColor=black)](https://www.buymeacoffee.com/mo3he)

Ad-hoc remote access tunnel for Axis devices, built on Tailscale's
[tailcat](https://github.com/tailscale/tailcat) data plane.

> **Experimental.** Upstream tailcat makes no API, CLI, or wire-format
> stability promises and is still pre-1.0. Treat this package as a
> pre-release and expect breaking changes between versions.

> **Disclaimer:** Independent, community-developed ACAP package. Not an official
> Axis product and not affiliated with, endorsed by, or supported by Axis
> Communications AB. Use at your own risk.

## Overview

Tailcat gives you a short-lived, encrypted route to a device that sits behind
NAT, without port forwarding, without a VPN, and without any account.

Start the tunnel and the app prints a **tailcat address**. Anyone you give that
address to can reach the device's own services through an end-to-end encrypted
WireGuard tunnel, NAT-traversed peer-to-peer where possible and relayed through
a DERP server when not.

This is not another VPN. It complements the persistent overlay apps
(Tailscale, NetBird, WireGuard, ZeroTier, OpenVPN) rather than replacing them:

| | Overlay VPN apps | Tailcat |
|---|---|---|
| Purpose | Join the device to a network permanently | Open a tunnel for a single job |
| Identity | Account, control plane, device registry | None, the address is the credential |
| Lifetime | Always on | Until you press Stop |
| Privilege | Often needs root | Runs unprivileged |

Typical use: an installer on site needs someone remote to look at a camera that
nobody will open ports for. Install, press Start, share the address, press Stop.

### How access is authenticated

The app **forwards ports to the device's own services** instead of
reimplementing them. Port 80/443 reaches the device web interface and port 22
reaches the device's own SSH server, so every connection is authenticated by
the device's normal credentials. The app adds no accounts of its own and needs
no root privilege.

## Compatibility

| Package | AXIS OS | Architecture | Status |
|---|---|---|---|
| ACAP 4 | 12.10.68 - 13 | aarch64 | Tested on hardware |
| ACAP 4 | 12.10.68 - 13 | armv7hf | Builds, **not yet run on a device** |

The app runs as the unprivileged `sdk` user. It does not require root access.

> **armv7hf is unverified.** It cross-compiles and packages correctly, and the
> binary is confirmed 32-bit ARM, but no armv7hf device was available to run it
> on. If you install it, please report whether it works.

## Installation

Download the EAP matching the device architecture from the
[Releases page](https://github.com/Mo3he/Axis_Cam_Tailcat/releases), then:

1. Open the device web interface.
2. Go to **Apps -> Add app**.
3. Upload the EAP and start the app.
4. Open the app settings, press **Start**, and copy the address.

## Connecting to the device

The device side is this ACAP. The client side is the
[tailcat CLI](https://github.com/tailscale/tailcat), which you install on the
machine you are connecting *from*:

```sh
brew install tailcat          # macOS
```

See [INSTALL.md](https://github.com/tailscale/tailcat/blob/main/INSTALL.md)
upstream for Linux, Windows and the other options.

Everything below takes the address you copied from the app. It is
**case sensitive**, and `forward` keeps running until you press Ctrl-C, so give
it a terminal of its own.

### Check you can reach it

```sh
tailcat ping <tc-addr>
```

Each reply says whether it arrived directly or through a relay. A relayed path
still works, it is just rate limited.

### Device web interface

```sh
tailcat forward <tc-addr> 8443:443
```

Then open `https://127.0.0.1:8443` and accept the certificate warning. The
device's certificate is issued for its own name, not `127.0.0.1`, so the
browser will always complain.

Axis devices redirect port 80 to HTTPS, and that redirect points at an address
your client cannot resolve. Use 443 as above. For the same reason
`tailcat browse`, which is a shortcut for port 80, is not useful here.

### SSH

```sh
tailcat ssh root@<tc-addr>
```

Requires SSH to be enabled on the device; it is off by default on AXIS OS 11
and later. The app's **Forwarded ports** panel shows whether port 22 is
actually listening.

### Video

```sh
tailcat forward <tc-addr> 8554:554
ffplay rtsp://root:PASSWORD@127.0.0.1:8554/axis-media/media.amp
```

Check the UI first and make sure the client is on a direct path. Relayed video
is unlikely to be watchable.

### Copying files

```sh
tailcat cp <tc-addr>:/path/to/file .
```

Uses the device's own SFTP over the tunnel, so the same credentials and the
same permissions apply as a local login.

## Configuration

Settings live in **Apps -> Tailcat -> Settings**.

| Setting | Default | Description |
|---|---|---|
| Forwarded TCP ports | `80, 443, 554, 22` | Ports proxied to this device's own services. |
| Start automatically | off | Open the tunnel as soon as the app starts. |
| Keep the same address | off | Persist the key so the address survives restarts. |
| Exit node | off | Let clients route to other hosts on the device's network. |
| Allowed client keys | empty | Restrict the tunnel to listed client node keys. |
| Auto-stop | off | Close the tunnel after inactivity or after a fixed time. |
| Custom DERP map URL | empty | Use your own relays instead of Tailscale's. |
| Verbose logging | off | Connection diagnostics to the system log. |

Port 554 is forwarded by default, but see the relay caveat below before
relying on it for video.

### Where settings are stored

Settings are stored as JSON in the app's `localdata` directory and served by
the app itself at `/local/Tailcat/api/settings`, reachable only through the
device's authenticated reverse proxy and restricted to `admin`.

Nothing uses `/axis-cgi/param.cgi`, so settings behave identically on recorder,
NVR, and access-control class devices that do not expose it. The trade-off is
that these settings are not in the device parameter store and therefore are not
included in a device configuration backup.

## Security

**The tailcat address is a credential.** It embeds the server's public key and
a WireGuard pre-shared key. Anyone holding it can reach the forwarded ports, so
share it only over a private channel. It is shown in the web UI and is
deliberately never written to the system log.

Defaults are deliberately conservative:

- The tunnel is stopped until you press Start.
- The key is ephemeral, so stopping the tunnel kills the address permanently.
- Exit node access is off.
- Only the listed ports are reachable; a packet filter drops everything else.

Consider setting **Allowed client keys** if you pin the address, so that
possession of the address alone is not sufficient to connect.

### Exit node

Exit node mode lets a client route traffic to other hosts on the device's
network. Enabling **full** access effectively makes the device a route into
that network for anyone holding the address. Prefer **restricted** mode with an
explicit list of subnets.

### Relays and video

If NAT traversal fails, traffic falls back to a DERP relay. Tailscale's public
relays are rate limited and offer no uptime guarantee, so RTSP and other video
is unlikely to be usable on a relayed path. Run [your own DERP
server](https://github.com/tailscale/tailscale/tree/main/cmd/derper#derp) and
set a custom DERP map URL if you need dependable throughput.

The web UI lists the clients that have connected, with active and total
connection counts, so an address shared further than intended is visible.
Whether a given path is direct or relayed is not shown: upstream's
`Server.Status` reports no peer detail, because it builds its status with
`WantPeers` unset.

## Ports and privileges

- The app opens no ports on the device's external interfaces.
- The settings endpoint binds `127.0.0.1:2208` and is reachable only through
  the device's authenticated reverse proxy, restricted to `admin`.
- Outbound connectivity is required: TCP 443 to DERP relays and UDP 3478 for
  STUN. Direct peer-to-peer paths use an ephemeral UDP port.

## Build from source

Docker or Podman is required; the Axis ACAP Native SDK image does the build.

```sh
./build.sh                 # aarch64 and armv7hf into ./releases
ARCHES=aarch64 ./build.sh  # a single architecture
```

The Go toolchain must be 1.27.1 or newer, which is what upstream tailcat
requires. The build derives its trim tags from the pinned tailcat module's own
`build-tags.txt` so they stay correct across upstream version bumps.

### CI

Every push builds both architectures and uploads the packages as workflow
artifacts. Pushing a `v*` tag creates a release with the packages attached.

## Links

- Releases: <https://github.com/Mo3he/Axis_Cam_Tailcat/releases>
- Issues: <https://github.com/Mo3he/Axis_Cam_Tailcat/issues>
- Upstream tailcat: <https://github.com/tailscale/tailcat>
- ACAP documentation: <https://developer.axis.com/acap/>
- Axis Communications: <https://www.axis.com/>

## Trademarks

Tailscale, Tailcat, and WireGuard are trademarks of their respective owners.
This project is independent and is not affiliated with, endorsed by, or
sponsored by Tailscale Inc.

## License

The packaging and app code in this repository is licensed under BSD 3-Clause
(see [LICENSE](LICENSE)). Bundled upstream components are listed in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
