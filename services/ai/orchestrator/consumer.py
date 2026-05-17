import asyncio
import json
import logging
import os
from nats.aio.client import Client as NATS
from orchestrator.graph import workflow_graph
from hub_ai.internal_client import HubInternalClient

logger = logging.getLogger(__name__)


# 全局 gRPC 客户端（惰性初始化）
_internal_client: HubInternalClient | None = None


def _get_internal_client() -> HubInternalClient:
    global _internal_client
    if _internal_client is None:
        _internal_client = HubInternalClient()
    return _internal_client


async def process_message(msg):
    task_id = ""
    try:
        data = json.loads(msg.data.decode())
        task_id = data.get("id")
        session_id = data.get("session_id", "")
        run_id = data.get("run_id", "")
        payload = data.get("payload", {})

        logger.info(f"Processing task {task_id} (run_id: {run_id})")

        # 构建初始状态
        initial_state = {
            "run_id": run_id,
            "user_input": payload.get("user_message", ""),
            "schema_context": payload.get("schema_json", "{}"),
        }
        config = {"configurable": {"thread_id": session_id or task_id}}

        # 在异步包装器内同步运行图，120s 超时
        result = await asyncio.wait_for(
            asyncio.to_thread(workflow_graph.invoke, initial_state, config),
            timeout=120.0
        )

        # 通过 gRPC 回调 Go API
        callback_result = {
            "final_report": result.get("final_report", ""),
            "analysis_summary": result.get("analysis_summary", "")
        }
        client = _get_internal_client()
        await asyncio.to_thread(
            client.task_callback,
            task_id=task_id,
            status="succeeded",
            result=callback_result,
        )
        logger.info(f"Task {task_id} callback successful via gRPC")
        await msg.ack()

    except Exception as e:
        logger.error(f"Task processing failed: {e}", exc_info=True)
        if task_id:
            try:
                client = _get_internal_client()
                await asyncio.to_thread(
                    client.task_callback,
                    task_id=task_id,
                    status="failed",
                    error_message=str(e),
                )
            except Exception as cb_err:
                logger.error(f"Failed to send failure callback via gRPC: {cb_err}")
        try:
            await msg.nak()
        except Exception:
            pass


async def run_consumer():
    nc = NATS()
    nats_url = os.environ.get("NATS_URL", "nats://localhost:4222")

    try:
        await nc.connect(nats_url)
        logger.info(f"Connected to NATS at {nats_url}")

        await nc.subscribe("hub.tasks.agent_pipeline", cb=process_message)

        while True:
            await asyncio.sleep(1)

    except Exception as e:
        logger.error(f"NATS connection error: {e}")
    finally:
        if nc.is_connected:
            await nc.drain()


if __name__ == '__main__':
    logging.basicConfig(level=logging.INFO)
    asyncio.run(run_consumer())
