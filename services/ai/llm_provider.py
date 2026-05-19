"""LLM Provider abstraction layer.

Defines the LLMProvider interface and built-in implementations:
- OpenAIProvider:  OpenAI-compatible API (default when OPENAI_API_KEY is set)
- FallbackProvider: Hardcoded fallback for development (no API key required)
- create_provider(): Factory function that selects the right provider

Usage:
    provider = create_provider()
    sql, notes = await provider.generate_sql(request)
    answer = await provider.ask(question, context)
"""

from __future__ import annotations

import json
import logging
import os
import re
from abc import ABC, abstractmethod
from typing import Any

logger = logging.getLogger(__name__)


class LLMProvider(ABC):
    """Abstract base class for LLM providers."""

    @abstractmethod
    async def generate_sql(
        self, request: Any, schema_json: str = ""
    ) -> tuple[str, str]:
        """Generate SQL from a natural language request.

        Returns:
            Tuple of (sql, notes).
        """
        ...

    @abstractmethod
    async def ask(self, question: str, context: str = "") -> str:
        """Answer a question using the LLM, optionally with RAG context."""
        ...


class OpenAIProvider(LLMProvider):
    """LLM provider using OpenAI-compatible API."""

    def __init__(self) -> None:
        from openai import AsyncOpenAI

        self._client = AsyncOpenAI(
            api_key=os.environ["OPENAI_API_KEY"],
            base_url=os.environ.get(
                "OPENAI_BASE_URL", "https://api.openai.com/v1"
            ),
        )
        self._model = os.environ.get("OPENAI_MODEL", "gpt-4o-mini")

    async def generate_sql(
        self, request: Any, schema_json: str = ""
    ) -> tuple[str, str]:
        """Generate MySQL SQL from a natural language request."""
        schema_text = self._format_schema(schema_json or "{}")
        prompt = (
            "You are a MySQL SQL generator. Reply ONLY with SQL, no markdown.\n"
            f"Tables:\n{schema_text}\n"
            f"Question: {request.user_message}\n"
            "Rules:\n"
            "- SELECT for queries;\n"
            "- INSERT/UPDATE/DELETE for data changes;\n"
            "- CREATE TABLE for new tables;\n"
            "- CRITICAL: NEVER fabricate data. If user asks to insert/update but "
            "does NOT provide specific values, reply with 'ERROR: ' followed by a "
            "Chinese question asking what data they want to provide. "
            "Example: user says '添加一条客户记录' → "
            "ERROR: 请提供要添加的客户信息，例如姓名、联系方式等\n"
            "- Only generate INSERT/UPDATE when user provides actual values;\n"
            "- NEVER touch system tables (users, workspaces, sessions, etc.);\n"
            "- Use MySQL backtick quoting for identifiers if needed."
        )
        r = await self._client.chat.completions.create(
            model=self._model,
            messages=[{"role": "user", "content": prompt}],
            temperature=0.1,
            timeout=60.0,
        )
        sql = (r.choices[0].message.content or "").strip()
        for prefix in ("```sql", "```"):
            if sql.startswith(prefix):
                sql = sql[len(prefix) :].strip()
        if sql.endswith("```"):
            sql = sql[:-3].strip()
        return sql, "openai"

    async def ask(self, question: str, context: str = "") -> str:
        """Answer a question using the LLM, optionally with RAG context."""
        if context:
            system_prompt = (
                "你是一个知识库问答助手。请基于以下提供的参考资料回答用户问题。\n"
                "如果参考资料不足以回答问题，请如实说明。\n"
                "请用中文回答。\n\n"
                "参考资料：\n"
                f"{context}"
            )
        else:
            system_prompt = (
                "你是一个知识库问答助手。请基于你的知识回答用户问题。\n"
                "如果不知道答案，请如实说明，不要编造。\n"
                "请用中文回答。"
            )

        r = await self._client.chat.completions.create(
            model=self._model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": question},
            ],
            temperature=0.1,
            timeout=30.0,
        )
        return (r.choices[0].message.content or "").strip()

    @staticmethod
    def _format_schema(schema_json: str) -> str:
        """Convert schema JSON to readable table descriptions (with truncation)."""
        MAX_CHARS = 6000
        try:
            schema = json.loads(schema_json)
        except (json.JSONDecodeError, TypeError):
            return "(no schema available)"

        tables = schema.get("tables", [])
        if not tables:
            return "(no tables discovered)"

        lines: list[str] = []
        total_chars = 0
        truncated = False
        for table in tables:
            name = table.get("name", "unknown")
            cols = table.get("columns", [])
            col_parts = []
            for col in cols:
                cname = col.get("name", "?")
                ctype = col.get("type", "text")
                col_parts.append(f"{cname} ({ctype})")
            line = f"- {name}: " + ", ".join(col_parts)
            total_chars += len(line) + 1
            if total_chars > MAX_CHARS and lines:
                truncated = True
                break
            lines.append(line)

        result = "\n".join(lines)
        if truncated:
            remaining = len(tables) - len(lines)
            result += f"\n(Schema truncated, {remaining} table(s) omitted)"
        return result


class FallbackProvider(LLMProvider):
    """Fallback provider for development when no API key is configured."""

    async def generate_sql(
        self, request: Any, schema_json: str = ""
    ) -> tuple[str, str]:
        msg = getattr(request, "user_message", request) if not isinstance(request, str) else request
        sql = "SELECT 1 AS ok"
        m = re.search(r"\d+", str(msg))
        if m:
            sql = f"SELECT {m.group()} AS n"
        return sql, "fallback (no OPENAI_API_KEY)"

    async def ask(self, question: str, context: str = "") -> str:
        return "No LLM provider configured"


def create_provider() -> LLMProvider:
    """Factory function: create the appropriate LLM provider based on environment."""
    if os.environ.get("OPENAI_API_KEY"):
        return OpenAIProvider()
    return FallbackProvider()
