import asyncio
import json
import logging
import os
from nats.aio.client import Client as NATS
from rag.knowledge_base import KnowledgeBase
from hub_ai.internal_client import HubInternalClient

logger = logging.getLogger(__name__)


# 全局 gRPC 客户端（惰性初始化）
_internal_client: HubInternalClient | None = None


def _get_internal_client() -> HubInternalClient:
    global _internal_client
    if _internal_client is None:
        _internal_client = HubInternalClient()
    return _internal_client


async def process_knowledge_message(msg):
    """处理知识索引任务：文档分块、嵌入、存入 ChromaDB。"""
    doc_id = ""
    chroma_doc_id = ""

    try:
        data = json.loads(msg.data.decode())
        payload = data.get("payload", {})
        doc_id = payload.get("doc_id", "")
        workspace_id = data.get("workspace_id", "")
        title = payload.get("title", "")
        content = payload.get("content", "")

        logger.info(f"Indexing document {doc_id} in workspace {workspace_id}")

        if not content or not workspace_id:
            logger.error("Missing content or workspace_id, skipping")
            await msg.ack()
            return

        # 为工作区初始化知识库
        kb = KnowledgeBase(workspace_id)

        # 分块和嵌入，获取 Chroma doc ID
        chroma_doc_id = kb.add_document(doc_id=doc_id, title=title, text_content=content)

        # 通过 gRPC 回调 Go API：成功
        chunk_count = 0  # kb.add_document may not return a count; try to extract
        client = _get_internal_client()
        await asyncio.to_thread(
            client.knowledge_doc_callback,
            doc_id=doc_id,
            status="completed",
            chroma_doc_id=chroma_doc_id,
            chunk_count=chunk_count,
        )
        logger.info(f"Document {doc_id} indexed successfully via gRPC")
        await msg.ack()

    except Exception as e:
        logger.error(f"Knowledge indexing failed for {doc_id}: {e}", exc_info=True)
        if doc_id:
            try:
                client = _get_internal_client()
                await asyncio.to_thread(
                    client.knowledge_doc_callback,
                    doc_id=doc_id,
                    status="failed",
                    chunk_count=0,
                )
            except Exception as cb_err:
                logger.error(f"Failed to send failure callback via gRPC: {cb_err}")
        try:
            await msg.nak()
        except Exception:
            pass


async def run_knowledge_consumer():
    nc = NATS()
    nats_url = os.environ.get("NATS_URL", "nats://localhost:4222")

    try:
        await nc.connect(nats_url)
        logger.info(f"Knowledge consumer connected to NATS at {nats_url}")

        await nc.subscribe("hub.tasks.knowledge_index", cb=process_knowledge_message)

        while True:
            await asyncio.sleep(1)

    except Exception as e:
        logger.error(f"Knowledge consumer NATS error: {e}")
    finally:
        if nc.is_connected:
            await nc.drain()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    asyncio.run(run_knowledge_consumer())
