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

## Fase 6: Stack Generation & Apply [IN PROGRESS]
- [ ] internal/stack/generate.go - Gerar docker-compose para stack mínima
- [ ] Stack isolada em /opt/orvyn/stack/
- [ ] Confirmação interativa obrigatória
- [ ] Backups automáticos
- [ ] Criar diretórios com permissões corretas
- [ ] Aplicar via docker compose
- [ ] Audit log
- [ ] Integrar com orvyn apply

## Fase 7: Validate & Status [PENDING]
- [ ] internal/validate/checks.go
- [ ] Verificar containers UP
- [ ] Verificar targets Prometheus
- [ ] Verificar Loki
- [ ] Integrar orvyn validate e orvyn status
