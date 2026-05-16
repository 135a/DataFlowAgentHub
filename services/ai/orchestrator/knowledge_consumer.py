import asyncio
import json
import logging
import os
import httpx
from nats.aio.client import Client as NATS
from rag.knowledge_base import KnowledgeBase
from hub_ai.shared import make_headers

logger = logging.getLogger(__name__)




async def process_knowledge_message(msg):
    """Process a knowledge indexing task: chunk document, embed, store in ChromaDB."""
    api_url = os.environ.get("HUB_API_INTERNAL_URL", "http://api:8080")
    secret = os.environ.get("HUB_INTERNAL_HMAC_SECRET", "dev-hmac-secret-change-me")
    doc_id = ""

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

        # Initialize KnowledgeBase for this workspace
        kb = KnowledgeBase(workspace_id)

        # Chunk and embed
        chunk_count = kb.add_document(doc_id=doc_id, title=title, text_content=content)

        # Callback to Go API: success
        cb_payload = {"status": "completed", "chunk_count": chunk_count}
        body_bytes = json.dumps(cb_payload).encode()
        async with httpx.AsyncClient() as client:
            resp = await client.patch(
                f"{api_url}/internal/knowledge-docs/{doc_id}/status",
                headers=make_headers(secret, body_bytes),
                content=body_bytes,
                timeout=10.0,
            )
            resp.raise_for_status()

        logger.info(f"Document {doc_id} indexed successfully ({chunk_count} chunks)")
        await msg.ack()

    except Exception as e:
        logger.error(f"Knowledge indexing failed for {doc_id}: {e}", exc_info=True)

        # Callback failure status
        if doc_id:
            try:
                cb_payload = {"status": "failed", "error_message": str(e)}
                body_bytes = json.dumps(cb_payload).encode()
                async with httpx.AsyncClient() as client:
                    await client.patch(
                        f"{api_url}/internal/knowledge-docs/{doc_id}/status",
                        headers=make_headers(secret, body_bytes),
                        content=body_bytes,
                        timeout=10.0,
                    )
            except Exception as cb_err:
                logger.error(f"Failed to send failure callback: {cb_err}")

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
