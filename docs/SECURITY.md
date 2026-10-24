# VPSMon Security Documentation

## Overview
VPSMon is local-first and privacy by default. Security is mandatory and enforced by design.

## Principles
- Local-first, no cloud dependency by default
- Least privilege
- Plan-first, explicit confirmation for changes
- Privacy by default, data minimization
- Defense in depth
- Secure by default

## Trust Boundaries
1. VPSMon process <-> OS
2. VPSMon <-> Docker daemon
3. VPSMon <-> filesystem/config
4. VPSMon <-> logs
5. VPSMon <-> external executables
6. VPSMon <-> dependencies

## Core Controls
- Secret redaction
- Path traversal/symlink protection
- No shell execution with concatenation
- Audit logging without secrets
- Backups before modifications
- Read-only discovery/plan
