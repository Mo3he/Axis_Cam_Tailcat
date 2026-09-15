# Build both architectures from this single Dockerfile:
#   docker build --build-arg ARCH=aarch64 --target package -t tailcat-aarch64 .
#   docker build --build-arg ARCH=armv7hf --target package -t tailcat-armv7hf .
#
# Prefer ./build.sh, which does both and collects the .eap files in ./releases.
ARG ARCH=aarch64
# tailcat's go.mod requires >= 1.27.1.
ARG GO_VERSION=1.27
ARG SDK_VERSION=12.10.0
ARG UBUNTU_VERSION=24.04
ARG SDK_REPO=axisecp
ARG SDK=acap-native-sdk

FROM --platform=linux/amd64 golang:${GO_VERSION} AS gobuilder
ARG ARCH
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY daemon ./daemon
# Trim tags come from the pinned tailcat module itself rather than a copy pasted
# here, so they stay correct across upstream bumps. ts_omit_ssh drops tailcat's
# built-in SSH server: this ACAP forwards to the camera's own sshd instead.
RUN set -eux; \
    tags="$(cat "$(go list -m -f '{{.Dir}}' github.com/tailscale/tailcat)/build-tags.txt"),ts_omit_ssh"; \
    if [ "${ARCH}" = "aarch64" ]; then export GOARCH=arm64; else export GOARCH=arm GOARM=7; fi; \
    GOOS=linux go build -mod=readonly -trimpath -tags="$tags" \
        -ldflags='-s -w' -o /out/Tailcat ./daemon

FROM ${SDK_REPO}/${SDK}:${SDK_VERSION}-${ARCH}-ubuntu${UBUNTU_VERSION} AS package
ARG ARCH
COPY app /opt/app/
COPY LICENSE /opt/app/LICENSE
COPY --from=gobuilder /out/Tailcat /opt/app/Tailcat
WORKDIR /opt/app
RUN sed -i "s/\"BUILDARCH\"/\"${ARCH}\"/" manifest.json && \
    find . -name .DS_Store -delete && \
    chmod 755 Tailcat && \
    . /opt/axis/acapsdk/environment-setup* && acap-build ./

FROM scratch
COPY --from=package /opt/app/*.eap /
