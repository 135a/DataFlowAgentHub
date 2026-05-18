"""gRPC client for calling Go API's HubInternalService over mTLS.

Replaces the old HTTP+HMAC callback pattern with direct gRPC calls.

Provides both async (grpc.aio) and synchronous (grpc) interfaces so it
can be used from async contexts (NATS consumers) as well as synchronous
contexts (LangGraph sync node functions running in a thread pool).
"""

from __future__ import annotations
import json
import logging
import os

import grpc
from grpc import aio as grpc_aio

from nl2sql.v1.nl2sql_pb2_grpc import HubInternalServiceStub
from nl2sql.v1.nl2sql_pb2 import (
    KnowledgeDocCallbackRequest,
    InternalNL2SQLRequest,
    RunStepCallbackRequest,
    TaskCallbackRequest,
)

logger = logging.getLogger(__name__)


class HubInternalClient:
    """Wraps HubInternalService gRPC stubs for all 4 callback RPCs.

    Maintains TWO channels internally:
    - ``_channel`` / ``_stub`` – synchronous (regular grpc), created in __init__
    - ``_channel_aio`` / ``_stub_aio`` – async (grpc.aio), created via await _connect()

    This allows the same client instance to be used from both sync and
    async callers without friction.

    Reads mTLS cert paths from environment variables. Falls back to insecure
    channel when cert env vars are not set (local dev).
    """

    def __init__(
        self,
        target: str | None = None,
        ca_cert: str | None = None,
        client_cert: str | None = None,
        client_key: str | None = None,
    ):
        self.target = target or os.getenv("HUB_GO_GRPC_TARGET", "api:9090")
        ca_cert = ca_cert or os.getenv("HUB_GRPC_CA_CERT") or ""
        client_cert = client_cert or os.getenv("HUB_GRPC_CLIENT_CERT") or ""
        client_key = client_key or os.getenv("HUB_GRPC_CLIENT_KEY") or ""

        # --- synchronous channel (regular grpc) ---
        if ca_cert and client_cert and client_key:
            logger.info(
                "creating sync gRPC channel with mTLS, target=%s", self.target
            )
            with open(ca_cert, "rb") as f:
                ca_bytes = f.read()
            with open(client_cert, "rb") as f:
                cert_bytes = f.read()
            with open(client_key, "rb") as f:
                key_bytes = f.read()
            creds = grpc.ssl_channel_credentials(
                root_certificates=ca_bytes,
                private_key=key_bytes,
                certificate_chain=cert_bytes,
            )
            self._channel = grpc.secure_channel(self.target, creds)
        else:
            logger.warning(
                "mTLS cert env vars not set, using insecure sync channel to %s",
                self.target,
            )
            self._channel = grpc.insecure_channel(self.target)
        self._stub = HubInternalServiceStub(self._channel)

        # --- async channel (grpc.aio) – created lazily via _connect() ---
        self._channel_aio: grpc_aio.Channel | None = None
        self._stub_aio: HubInternalServiceStub | None = None

    async def _connect(self) -> None:
        """Create the async gRPC channel and stub (idempotent)."""
        if self._channel_aio is not None:
            return
        ca_cert = os.getenv("HUB_GRPC_CA_CERT") or ""
        client_cert = os.getenv("HUB_GRPC_CLIENT_CERT") or ""
        client_key = os.getenv("HUB_GRPC_CLIENT_KEY") or ""
        if ca_cert and client_cert and client_key:
            logger.info(
                "creating async gRPC channel with mTLS, target=%s", self.target
            )
            with open(ca_cert, "rb") as f:
                ca_bytes = f.read()
            with open(client_cert, "rb") as f:
                cert_bytes = f.read()
            with open(client_key, "rb") as f:
                key_bytes = f.read()
            creds = grpc_aio.ssl_channel_credentials(
                root_certificates=ca_bytes,
                private_key=key_bytes,
                certificate_chain=cert_bytes,
            )
            self._channel_aio = grpc_aio.secure_channel(self.target, creds)
        else:
            logger.warning(
                "mTLS cert env vars not set, using insecure async channel to %s",
                self.target,
            )
            self._channel_aio = grpc_aio.insecure_channel(self.target)
        self._stub_aio = HubInternalServiceStub(self._channel_aio)

    # ------------------------------------------------------------------
    # TaskCallback  – async
    # ------------------------------------------------------------------
    async def task_callback(
        self,
        task_id: str,
        status: str,
        result: dict | list | None = None,
        error_message: str = "",
    ) -> None:
        """Report async task result to Go API (async)."""
        result_json = json.dumps(result) if result is not None else "null"
        req = TaskCallbackRequest(
            task_id=task_id,
            status=status,
            result_json=result_json,
            error_message=error_message,
        )
        try:
            await self._stub_aio.TaskCallback(req)
        except grpc_aio.AioRpcError as e:
            logger.error("TaskCallback RPC failed: %s", e)

    # ------------------------------------------------------------------
    # TaskCallback  – synchronous wrapper (for sync contexts)
    # ------------------------------------------------------------------
    def task_callback_sync(
        self,
        task_id: str,
        status: str,
        result: dict | list | None = None,
        error_message: str = "",
    ) -> None:
        """Synchronous version of task_callback."""
        result_json = json.dumps(result) if result is not None else "null"
        req = TaskCallbackRequest(
            task_id=task_id,
            status=status,
            result_json=result_json,
            error_message=error_message,
        )
        try:
            self._stub.TaskCallback(req)
        except grpc.RpcError as e:
            logger.error("TaskCallback RPC failed: %s", e)

    # ------------------------------------------------------------------
    # RunStepCallback  – async
    # ------------------------------------------------------------------
    async def run_step_callback(
        self,
        run_id: str,
        agent_name: str,
        status: str,
        input_summary: str = "",
        output_summary: str = "",
        error_message: str = "",
    ) -> None:
        """Report a LangGraph agent step to Go API (async)."""
        MAX_SUMMARY = 1000
        req = RunStepCallbackRequest(
            run_id=run_id,
            agent_name=agent_name,
            status=status,
            input_summary=input_summary[:MAX_SUMMARY],
            output_summary=output_summary[:MAX_SUMMARY],
            error_message=error_message,
        )
        try:
            await self._stub_aio.RunStepCallback(req)
        except grpc_aio.AioRpcError as e:
            logger.warning("RunStepCallback RPC failed: %s", e)

    # ------------------------------------------------------------------
    # RunStepCallback  – synchronous wrapper
    # ------------------------------------------------------------------
    def run_step_callback_sync(
        self,
        run_id: str,
        agent_name: str,
        status: str,
        input_summary: str = "",
        output_summary: str = "",
        error_message: str = "",
    ) -> None:
        """Synchronous version of run_step_callback."""
        MAX_SUMMARY = 1000
        req = RunStepCallbackRequest(
            run_id=run_id,
            agent_name=agent_name,
            status=status,
            input_summary=input_summary[:MAX_SUMMARY],
            output_summary=output_summary[:MAX_SUMMARY],
            error_message=error_message,
        )
        try:
            self._stub.RunStepCallback(req)
        except grpc.RpcError as e:
            logger.warning("RunStepCallback RPC failed: %s", e)

    # ------------------------------------------------------------------
    # InternalNL2SQL  – async
    # ------------------------------------------------------------------
    async def internal_nl2sql(
        self,
        user_message: str,
        schema_json: str = "",
        trace_id: str = "",
        dialect: str = "postgres",
    ) -> dict:
        """Call Go's NL2SQL executor (async)."""
        req = InternalNL2SQLRequest(
            trace_id=trace_id,
            user_message=user_message,
            schema_json=schema_json,
            dialect=dialect,
        )
        resp = await self._stub_aio.InternalNL2SQL(req)
        if not resp.ok:
            return {"ok": False, "error_message": resp.error_message}
        return {
            "ok": True,
            "sql": resp.sql,
            "rows": json.loads(resp.rows_json) if resp.rows_json else [],
            "notes": resp.notes,
        }

    # ------------------------------------------------------------------
    # InternalNL2SQL  – synchronous wrapper
    # ------------------------------------------------------------------
    def internal_nl2sql_sync(
        self,
        user_message: str,
        schema_json: str = "",
        trace_id: str = "",
        dialect: str = "postgres",
    ) -> dict:
        """Synchronous version of internal_nl2sql."""
        req = InternalNL2SQLRequest(
            trace_id=trace_id,
            user_message=user_message,
            schema_json=schema_json,
            dialect=dialect,
        )
        resp = self._stub.InternalNL2SQL(req)
        if not resp.ok:
            return {"ok": False, "error_message": resp.error_message}
        return {
            "ok": True,
            "sql": resp.sql,
            "rows": json.loads(resp.rows_json) if resp.rows_json else [],
            "notes": resp.notes,
        }

    # ------------------------------------------------------------------
    # KnowledgeDocCallback  – async
    # ------------------------------------------------------------------
    async def knowledge_doc_callback(
        self,
        doc_id: str,
        status: str,
        chroma_doc_id: str = "",
        chunk_count: int = 0,
    ) -> None:
        """Report knowledge document indexing status to Go API (async)."""
        req = KnowledgeDocCallbackRequest(
            doc_id=doc_id,
            status=status,
            chroma_doc_id=chroma_doc_id,
            chunk_count=chunk_count,
        )
        try:
            await self._stub_aio.KnowledgeDocCallback(req)
        except grpc_aio.AioRpcError as e:
            logger.error("KnowledgeDocCallback RPC failed: %s", e)

    # ------------------------------------------------------------------
    # KnowledgeDocCallback  – synchronous wrapper
    # ------------------------------------------------------------------
    def knowledge_doc_callback_sync(
        self,
        doc_id: str,
        status: str,
        chroma_doc_id: str = "",
        chunk_count: int = 0,
    ) -> None:
        """Synchronous version of knowledge_doc_callback."""
        req = KnowledgeDocCallbackRequest(
            doc_id=doc_id,
            status=status,
            chroma_doc_id=chroma_doc_id,
            chunk_count=chunk_count,
        )
        try:
            self._stub.KnowledgeDocCallback(req)
        except grpc.RpcError as e:
            logger.error("KnowledgeDocCallback RPC failed: %s", e)

    # ------------------------------------------------------------------
    # 资源清理
    # ------------------------------------------------------------------
    async def close(self) -> None:
        """Close both sync and async channels."""
        self._channel.close()
        if self._channel_aio:
            await self._channel_aio.close()
            self._channel_aio = None
            self._stub_aio = None
