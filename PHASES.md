# Orvyn - Roadmap por Fases

## Fase 0: Fundação & Git [COMPLETED]
- [x] Inicializar repositório Git
- [x] Criar .gitignore
- [x] Criar README.md
- [x] Criar SECURITY.md
- [x] Criar PHASES.md
- [x] Compilar com sucesso

## Fase 1: Core de Segurança [COMPLETED]
- [x] internal/fsutil/ - SafeJoin, path traversal protection
- [x] internal/redaction/ - Redação de segredos
- [x] internal/audit/ - Logging de eventos sem segredos
- [x] internal/state/ - Gestão de estado com permissões restritas
- [x] internal/config/ - Configuração com paths seguros
- [x] Testes unitários para fsutil, redaction, audit

## Fase 2: Docker Wrapper (read-only) [COMPLETED]
- [x] internal/docker/client.go - Cliente com timeouts
- [x] Ping(), Info(), ListContainers()
- [x] ContainerLogs() preparado para streaming
- [x] Tratamento de erros seguro

## Fase 3: Discovery (read-only) [COMPLETED]
- [x] internal/discovery/discover.go
- [x] Detectar SO, Docker, containers
- [x] Detectar projetos Docker Compose
- [x] Detectar stack existente
- [x] Integrar com orvyn discover

## Fase 4: Logs no Terminal [COMPLETED]
- [x] internal/logs/stream.go - Streaming via Docker API
- [x] Integração com redaction
- [x] Flags: --follow, --tail, --timestamps
- [x] Resolução de serviço/container
- [x] Integrar com orvyn logs

## Fase 5: Plan Engine [COMPLETED]
- [x] internal/planner/plan.go
- [x] Comparar discovered vs desired
- [x] Gerar ações detalhadas
- [x] Identificar backups necessários
- [x] Integrar com orvyn plan
- [x] Plan state-aware (mostra `ok` quando não há alterações)

## Fase 6: Stack Generation & Apply [COMPLETED]
- [x] internal/stack/generate.go - Gerar docker-compose para stack mínima
- [x] Stack isolada em paths user-local ou system (UID-aware)
- [x] Confirmação interativa obrigatória
- [x] Backups automáticos
- [x] Criar diretórios com permissões corretas
- [x] Aplicar via docker compose
- [x] Audit log
- [x] Integrar com orvyn apply
- [x] Imagens pinadas (sem `:latest`)
- [x] Limites de recursos host-aware (mem_limit, cpus, pids_limit)
- [x] Apply session tracking com recuperação de falhas parciais

## Fase 7: Validate & Status [COMPLETED]
- [x] internal/validate/preflight.go
- [x] Verificar containers UP
- [x] Verificar targets Prometheus
- [x] Verificar Loki
- [x] Integrar orvyn validate e orvyn status
- [x] Health via `docker inspect` (healthcheck status)
- [x] orvyn doctor e orvyn info

## Fase 8: Lifecycle & Service Control [COMPLETED]
- [x] orvyn up (idempotente) e orvyn ps
- [x] orvyn start / stop / restart (stack inteira ou serviço)
- [x] orvyn down / rm (volumes preservados por default)
- [x] orvyn uninstall (confirmação + --force/--volumes)
- [x] orvyn exec em containers geridos
- [x] Label-based targeting (`com.orvyn.managed=true`)

## Fase 9: Maintenance [COMPLETED]
- [x] orvyn prune
- [x] orvyn upgrade
- [x] Backup real + `orvyn restore`

## Fase 10: Hardening de Segurança [COMPLETED]
- [x] executil allowlist para todas as chamadas externas
- [x] Prometheus/Loki ligados apenas a 127.0.0.1
- [x] Redaction aplicada em logs e audit
- [x] Fluxos destrutivos com confirmação explícita
- [x] Paths user-local por omissão (UID-aware)

## Fase 11: CI/CD [COMPLETED]
- [x] Permissões mínimas (`contents: read`)
- [x] Actions pinadas por SHA
- [x] `concurrency` com cancel-in-progress
- [x] `timeout-minutes` por job
- [x] gofmt, vet, build, test
- [x] staticcheck (não bloqueante)
- [ ] govulncheck - suspenso até existir correção para GO-2026-4887/GO-2026-4883 (docker/docker v28.5.2)

## Fase 12: TUI Dashboard [COMPLETED]
- [x] internal/tui - Bubble Tea / Lip Gloss / Bubbles
- [x] Views: Overview, Logs, Metrics, Details, Help
- [x] Filtro incremental (`/`)
- [x] Ações rápidas: start / stop / restart com confirmação
- [x] Métricas de host (`/proc`) e de containers (Docker stats)
- [x] Health cache via `docker inspect`
- [x] Navegação por teclado estilo k9s

## Fase 13: Backup & Restore [COMPLETED]
- [x] `internal/backup` - arquivo tar.gz com timestamp em `BackupDir`
- [x] `orvyn backup` (com `--list`)
- [x] `orvyn restore [backup]` (`--latest`, `--yes`, confirmação interativa)
- [x] Backup automático no `apply` antes de escrever ficheiros
- [x] Restore fail-closed: valida o arquivo inteiro num diretório temporário antes de mover
- [x] Rejeição de path traversal, caminhos absolutos, symlinks, roots desconhecidos e entradas acima do limite

## Próximos passos
- [ ] Reintroduzir `govulncheck` scoped (`./cmd/... ./internal/... ./pkg/...`) quando docker/docker tiver correção
- [ ] Testes de integração para o TUI e para `internal/docker`
- [ ] Exportar métricas do próprio Orvyn (opcional)
