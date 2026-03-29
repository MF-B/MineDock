# TODO Index

Generated at: 2026-03-30T00:04:01+08:00
Source pattern: TODO(username): description
Repository root: C:/Users/guzem/Desktop/Core/毕设/MineDock

## Summary
- Total TODO items: 9

## Items
- [ ] minedock: Extract a shared handler flow for start/stop/delete. (backend/internal/api/handlers.go:77)
- [ ] minedock: Add centralized encoding-error logging for better observability. (backend/internal/api/handlers.go:144)
- [ ] minedock: Make create and save behavior atomic across Docker and SQLite. (backend/internal/service/docker_service.go:45)
- [ ] minedock: Replace per-item Save with a batched or transactional sync path. (backend/internal/service/docker_service.go:109)
- [ ] minedock: Guard concurrent snapshot writes to avoid last-write-wins races. (backend/internal/service/docker_service.go:110)
- [ ] minedock: Report sync failures without breaking the listing path. (backend/internal/service/docker_service.go:137)
- [ ] minedock: Split this function into store read and Docker reconcile helpers. (backend/internal/service/docker_service.go:166)
- [ ] minedock: Revisit connection strategy when write throughput requirements grow. (backend/internal/store/memory.go:42)
- [ ] minedock: Replace text matching with SQLite error-code based detection. (backend/internal/store/memory.go:165)
