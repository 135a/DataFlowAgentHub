import asyncio
import json
import logging
import os
from nats.aio.client import Client as NATS
from rag.knowledge_base import KnowledgeBase
from hub_ai._client import get_client

logger = logging.getLogger(__name__)


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

        logger.info("Indexing document %s in workspace %s", doc_id, workspace_id)

        if not content or not workspace_id:
            logger.error("Missing content or workspace_id, skipping")
            await msg.ack()
            return

        # 为工作区初始化知识库
        kb = KnowledgeBase(workspace_id)

        # 分块和嵌入，获取 Chroma doc ID
        chroma_doc_id = kb.add_document(doc_id=doc_id, title=title, text_content=content)

        # 通过 gRPC 回调 Go API：成功（异步，无需 to_thread）
        client = await get_client()
        await client.knowledge_doc_callback(
            doc_id=doc_id,
            status="completed",
            chroma_doc_id=chroma_doc_id,
            chunk_count=0,
        )
        logger.info("Document %s indexed successfully via gRPC", doc_id)
        await msg.ack()

    except Exception as e:
        logger.error("Knowledge indexing failed for %s: %s", doc_id, e, exc_info=True)
        if doc_id:
            try:
                client = await get_client()
                await client.knowledge_doc_callback(
                    doc_id=doc_id,
                    status="failed",
                    chunk_count=0,
                )
            except Exception as cb_err:
                logger.error("Failed to send failure callback via gRPC: %s", cb_err)
        try:
            await msg.nak()
        except Exception:
            pass


async def run_knowledge_consumer(nats_url=None):
    max_retries = 5
    retry_delay = 1
    nats_url = nats_url or os.environ.get("NATS_URL", "nats://localhost:4222")

    for attempt in range(max_retries):
        nc = NATS()
        try:
            await nc.connect(nats_url)
            logger.info("Knowledge consumer connected to NATS at %s", nats_url)

            await nc.subscribe("hub.tasks.knowledge_index", cb=process_knowledge_message)
            logger.info("Subscribed to hub.tasks.knowledge_index")

            await asyncio.Future()  # 保持运行

        except asyncio.CancelledError:
            logger.info("Knowledge NATS consumer cancelled")
            if nc.is_connected:
                await nc.drain()
            break
        except Exception as e:
            logger.error(
                "Knowledge NATS connection attempt %d/%d failed: %s",
                attempt + 1, max_retries, e,
            )
            if nc.is_connected:
                await nc.drain()
            if attempt < max_retries - 1:
                await asyncio.sleep(retry_delay)
                retry_delay = min(retry_delay * 2, 30)  # 指数退避
            else:
                logger.error("All Knowledge NATS reconnection attempts exhausted, giving up")
                raise


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    asyncio.run(run_knowledge_consumer())
