from __future__ import annotations

from typing import Protocol


class VectorStore(Protocol):
    def search(self, tenant_id: str, query: str, top_k: int = 8) -> list[dict[str, object]]: ...


class PgVectorStore:
    def search(self, tenant_id: str, query: str, top_k: int = 8) -> list[dict[str, object]]:
        _ = (tenant_id, query, top_k)
        return []
