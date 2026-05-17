"""gRPC client for calling Go API's HubInternalService over mTLS.

Replaces the old HTTP+HMAC callback pattern with direct gRPC calls.
"""

import json
import logging
import os

import grpc

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

        if ca_cert and client_cert and client_key:
            logger.info(
                "connecting to Go gRPC with mTLS, target=%s", self.target
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
                "mTLS cert env vars not set, using insecure channel to %s",
                self.target,
            )
            self._channel = grpc.insecure_channel(self.target)

        self._stub = HubInternalServiceStub(self._channel)

    # ------------------------------------------------------------------
    # TaskCallback  – 报告异步任务结果（succeeded / failed）
    # ------------------------------------------------------------------
    def task_callback(
        self,
        task_id: str,
        status: str,
        result: dict | list | None = None,
        error_message: str = "",
    ) -> None:
        """Report async task result to Go API."""
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
    # RunStepCallback – 追踪 LangGraph 步骤
    # ------------------------------------------------------------------
    def run_step_callback(
        self,
        run_id: str,
        agent_name: str,
        status: str,
        input_summary: str = "",
        output_summary: str = "",
        error_message: str = "",
    ) -> None:
        """Report a LangGraph agent step to Go API."""
        # truncate long summaries to keep the payload reasonable
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
    # InternalNL2SQL – 调用 Go 安全边界执行 NL2SQL
    # ------------------------------------------------------------------
    def internal_nl2sql(
        self,
        user_message: str,
        schema_json: str = "",
        trace_id: str = "",
        dialect: str = "postgres",
    ) -> dict:
        """Call Go's NL2SQL executor (SQL generation → read-only execution).

        Returns a dict with keys: sql, rows, notes, ok, error_message.
        """
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
    # KnowledgeDocCallback – 报告文档索引状态
    # ------------------------------------------------------------------
    def knowledge_doc_callback(
        self,
        doc_id: str,
        status: str,
        chroma_doc_id: str = "",
        chunk_count: int = 0,
    ) -> None:
        """Report knowledge document indexing status to Go API."""
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
    def close(self) -> None:
        self._channel.close()
