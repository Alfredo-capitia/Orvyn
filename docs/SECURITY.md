# Orvyn Security Documentation

## Overview
Orvyn is local-first and privacy by default. Security is mandatory and enforced by design.

## Principles
- Local-first, no cloud dependency by default
- Least privilege
- Plan-first, explicit confirmation for changes
- Privacy by default, data minimization
- Defense in depth
- Secure by default

## Trust Boundaries
1. Orvyn process <-> OS
2. Orvyn <-> Docker daemon
3. Orvyn <-> filesystem/config
4. Orvyn <-> logs
5. Orvyn <-> external executables
6. Orvyn <-> dependencies

## Core Controls
- Secret redaction
- Path traversal/symlink protection
- No shell execution with concatenation
- Audit logging without secrets
- Backups before modifications
- Read-only discovery/plan
