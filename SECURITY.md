# Security Policy

## Threat model

The HERO9 Black exposes its HTTP API over the USB link **without any authentication**.
Anything that can reach the camera's private subnet (`172.2x.1xx.0/24`, USB only) can
start recording, change settings, delete all media and upload firmware. `gopro9` adds no
authentication because the camera offers none. The `--yes` gates and confirmation prompts on
destructive commands protect against operator mistakes, not against an attacker on the link.

The camera enforces a `Host` header check (DNS-rebinding guard). `gopro9` always sets `Host`
to the camera IP; it never proxies or exposes the camera on another interface.

## Supported versions

Only the latest minor release receives security fixes.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting on this repository
(**Security → Report a vulnerability**). Do not open a public issue. You will receive an
acknowledgement within 72 hours and a fix or mitigation plan within 14 days for confirmed issues.
