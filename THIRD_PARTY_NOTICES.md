# Third-party notices

The packaging and app code in this repository is licensed under the BSD 3-Clause
License; bundled upstream components retain their own licenses.

## Axis ACAP Native SDK

- Project: <https://developer.axis.com/acap/>
- Used at build time to compile and package the application.

## tailcat

- Project: <https://github.com/tailscale/tailcat>
- License: BSD-3-Clause.
- Version: see the `github.com/tailscale/tailcat` requirement in `go.mod`.
- Linked into the application binary and provides the tunnel data plane.

```text
Copyright (c) Tailscale Inc & AUTHORS.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software
   without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## tailscale.com

- Project: <https://github.com/tailscale/tailscale>
- License: BSD-3-Clause.
- Version: see the `tailscale.com` requirement in `go.mod`.
- Pulled in by tailcat; supplies magicsock, DERP, and the userspace WireGuard
  and netstack implementations.

## gVisor

- Project: <https://github.com/google/gvisor>
- License: Apache-2.0.
- Pulled in transitively via `tailscale.com` and provides the userspace TCP/IP
  stack (`gvisor.dev/gvisor/pkg/tcpip`).

## wireguard-go

- Project: <https://git.zx2c4.com/wireguard-go/>
- License: MIT.
- Pulled in transitively and provides the userspace WireGuard implementation.

A complete, resolved dependency list with versions is in `go.sum` and can be
produced with `go list -m all`.

## Trademarks

All product names, logos, and brands are property of their respective owners.
Tailscale and Tailcat are trademarks of Tailscale Inc. WireGuard is a
registered trademark of Jason A. Donenfeld. This is an independent community
project and is not affiliated with or endorsed by any of them.
