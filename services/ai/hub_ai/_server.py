"""gRPC servicer for the NL2SQLService RPCs.

Extracted from __main__.py for testability. All methods use async
signatures compatible with grpc.aio server.

The servicer handles five RPCs:
- GenerateSQL:  NL2SQL via LLM provider (OpenAI or fallback)
- RunAgentPipeline:  Multi-agent LangGraph pipeline
- Health:  Service health check
- RAGSearch:  ChromaDB retrieval-augmented generation
- IndexDocument:  Document chunking, embedding, and ChromaDB storage
"""

from __future__ import annotations
import json
import logging

import grpc
from grpc import aio as grpc_aio

from nl2sql.v1 import nl2sql_pb2, nl2sql_pb2_grpc

from llm_provider import create_provider

logger = logging.getLogger(__name__)

_WORKER_VERSION = "0.1.0"


class HubAIServicer(nl2sql_pb2_grpc.NL2SQLServiceServicer):
    """gRPC servicer implementing NL2SQLService.

    All public RPC handlers use async def so they can be served by
    a grpc.aio server (see __main__.py).
    """

    def __init__(self) -> None:
        self._llm = create_provider()

    async def Health(
        self, request: nl2sql_pb2.HealthRequest, context: grpc_aio.ServicerContext
    ) -> nl2sql_pb2.HealthResponse:
        return nl2sql_pb2.HealthResponse(version=_WORKER_VERSION, ok=True)

    async def GenerateSQL(
        self, request: nl2sql_pb2.GenerateSQLRequest, context: grpc_aio.ServicerContext
    ) -> nl2sql_pb2.GenerateSQLResponse:
        md = dict(context.invocation_metadata())
        trace = request.trace_id or md.get("x-trace-id") or ""
        logger.info(
            json.dumps(
                {
                    "event": "generate_sql",
                    "trace_id": trace,
                    "session_id": request.session_id,
                }
            )
        )
        sql, notes = await self._llm.generate_sql(request, request.schema_json)

        if sql.startswith("ERROR:"):
            return nl2sql_pb2.GenerateSQLResponse(
                ok=False, error_message=sql[6:].strip(), sql="", self_check_notes=""
            )
        return nl2sql_pb2.GenerateSQLResponse(
            ok=True, sql=sql, self_check_notes=notes, error_message=""
        )

    async def RunAgentPipeline(
        self,
        request: nl2sql_pb2.RunAgentPipelineRequest,
        context: grpc_aio.ServicerContext,
    ) -> nl2sql_pb2.RunAgentPipelineResponse:
        md = dict(context.invocation_metadata())
        trace = request.trace_id or md.get("x-trace-id") or ""
        logger.info(
            json.dumps(
                {
                    "event": "run_agent_pipeline",
                    "trace_id": trace,
                    "session_id": request.session_id,
                    "run_id": request.run_id,
                }
            )
        )

        try:
            from orchestrator.graph import workflow_graph

            initial_state = {
                "run_id": request.run_id,
                "user_input": request.user_message,
                "schema_context": request.schema_json,
            }
            config = {"configurable": {"thread_id": request.session_id}}
            result = await workflow_graph.ainvoke(initial_state, config=config)

            return nl2sql_pb2.RunAgentPipelineResponse(
                ok=True,
                error_message="",
                final_report=result.get("final_report", ""),
            )
        except Exception as e:
            logger.error("Pipeline failed: %s", e)
            return nl2sql_pb2.RunAgentPipelineResponse(
                ok=False, error_message=str(e), final_report=""
            )

    # ------------------------------------------------------------------
    # RAG / Knowledge Base methods
    # ------------------------------------------------------------------

    async def RAGSearch(
        self, request: nl2sql_pb2.RAGSearchRequest, context: grpc_aio.ServicerContext
    ) -> nl2sql_pb2.RAGSearchResponse:
        """ChromaDB 检索增强生成：检索相关文档片段 + LLM 生成回答。"""
        md = dict(context.invocation_metadata())
        trace = request.trace_id or md.get("x-trace-id") or ""
        logger.info(
            json.dumps({
                "event": "rag_search",
                "trace_id": trace,
                "workspace_id": request.workspace_id,
                "question": request.question,
            })
        )

        try:
            from rag.knowledge_base import KnowledgeBase

            kb = KnowledgeBase(request.workspace_id)
            top_k = request.top_k or 3
            snippets = kb.search(request.question, top_k=top_k)

            if not snippets:
                # No relevant documents found — LLM answers based on own knowledge
                answer = await self._llm.ask(request.question, "")
                return nl2sql_pb2.RAGSearchResponse(
                    ok=True,
                    answer=answer,
                    sources=[],
                    error_message="",
                )

            # Format context from retrieved snippets
            context_lines = []
            for i, sn in enumerate(snippets):
                title = sn.get("metadata", {}).get("title", "unknown")
                content = sn.get("content", "")
                context_lines.append(f"[Source {i + 1}] {title}\n{content}")
            context_str = "\n\n---\n\n".join(context_lines)

            answer = await self._llm.ask(request.question, context_str)

            sources = [
                nl2sql_pb2.ChunkSource(
                    content=sn.get("content", ""),
                    doc_id=sn.get("metadata", {}).get("doc_id", ""),
                    title=sn.get("metadata", {}).get("title", ""),
                    distance=sn.get("distance", 0.0),
                    chunk_index=sn.get("metadata", {}).get("chunk_index", 0),
                )
                for sn in snippets
            ]

            return nl2sql_pb2.RAGSearchResponse(
                ok=True, answer=answer, sources=sources, error_message=""
            )
        except Exception as e:
            logger.error("RAGSearch failed: %s", e)
            return nl2sql_pb2.RAGSearchResponse(
                ok=False, answer="", sources=[], error_message=str(e)
            )

    async def IndexDocument(
        self, request: nl2sql_pb2.IndexDocumentRequest, context: grpc_aio.ServicerContext
    ) -> nl2sql_pb2.IndexDocumentResponse:
        """文档分块、生成 embedding 并存入 ChromaDB。"""
        md = dict(context.invocation_metadata())
        trace = request.trace_id or md.get("x-trace-id") or ""
        logger.info(
            json.dumps({
                "event": "index_document",
                "trace_id": trace,
                "workspace_id": request.workspace_id,
                "doc_id": request.doc_id,
                "title": request.title,
            })
        )

        try:
            from rag.knowledge_base import KnowledgeBase

            kb = KnowledgeBase(request.workspace_id)
            chunk_count = kb.add_document(
                doc_id=request.doc_id,
                title=request.title,
                text_content=request.text_content,
            )
            return nl2sql_pb2.IndexDocumentResponse(
                ok=True, chunk_count=chunk_count, error_message=""
            )
        except Exception as e:
            logger.error("IndexDocument failed: %s", e)
            return nl2sql_pb2.IndexDocumentResponse(
                ok=False, chunk_count=0, error_message=str(e)
            )

