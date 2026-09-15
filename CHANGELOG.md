# Changelog

## Unreleased

## 0.6.0

- Initial pre-release, tracking upstream tailcat v0.6.0.
- Forwards selected TCP ports (default 80, 443, 554, 22) over Tailscale's data
  plane with no account, no control plane, and no inbound ports.
- Forwards to the device's own services, so connections are authenticated by
  the device's existing credentials and the app needs no root privilege.
- Ephemeral key by default; optional pinned address persisted 0600 in
  localdata and never placed in the device parameter store.
- Optional exit node, off by default, with a restricted mode limited to listed
  subnets.
- Optional client key allow-list.
- Optional auto-stop after inactivity or after a fixed time.
- Web UI shows the address, connected clients, and whether each path is direct
  or relayed.
- The tailcat address is never written to the system log.
