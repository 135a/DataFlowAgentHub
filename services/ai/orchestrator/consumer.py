from __future__ import annotations
import asyncio
import json
import logging
import os
from nats.aio.client import Client as NATS
from orchestrator.graph import workflow_graph
from hub_ai._client import get_client

logger = logging.getLogger(__name__)


async def process_message(msg):
    task_id = ""
    try:
        data = json.loads(msg.data.decode())
        task_id = data.get("id")
        session_id = data.get("session_id", "")
        run_id = data.get("run_id", "")
        payload = data.get("payload", {})

        logger.info("Processing task %s (run_id: %s)", task_id, run_id)

        # 构建初始状态
        initial_state = {
            "run_id": run_id,
            "user_input": payload.get("user_message", ""),
            "schema_context": payload.get("schema_json", "{}"),
        }
        config = {"configurable": {"thread_id": session_id or task_id}}

        # 异步运行 LangGraph（ainvoke）, 120s 超时
        result = await asyncio.wait_for(
            workflow_graph.ainvoke(initial_state, config),
            timeout=120.0,
        )

        # 通过 gRPC 回调 Go API（异步，无需 to_thread）
        callback_result = {
            "final_report": result.get("final_report", ""),
            "analysis_summary": result.get("analysis_summary", ""),
        }
        client = await get_client()
        await client.task_callback(
            task_id=task_id,
            status="succeeded",
            result=callback_result,
        )
        logger.info("Task %s callback successful via gRPC", task_id)
        await msg.ack()

    except Exception as e:
        logger.error("Task processing failed: %s", e, exc_info=True)
        if task_id:
            try:
                client = await get_client()
                await client.task_callback(
                    task_id=task_id,
                    status="failed",
                    error_message=str(e),
                )
            except Exception as cb_err:
                logger.error("Failed to send failure callback via gRPC: %s", cb_err)
        try:
            await msg.nak()
        except Exception:
            pass


async def run_consumer(nats_url=None):
    max_retries = 5
    retry_delay = 1
    nats_url = nats_url or os.environ.get("NATS_URL", "nats://localhost:4222")

    for attempt in range(max_retries):
        nc = NATS()
        try:
            await nc.connect(nats_url)
            logger.info("Connected to NATS at %s", nats_url)

            await nc.subscribe("hub.tasks.agent_pipeline", cb=process_message)
            logger.info("Subscribed to hub.tasks.agent_pipeline")

            await asyncio.Future()  # 保持运行

        except asyncio.CancelledError:
            logger.info("NATS consumer cancelled")
            if nc.is_connected:
                await nc.drain()
            break
        except Exception as e:
            logger.error(
                "NATS connection attempt %d/%d failed: %s",
                attempt + 1, max_retries, e,
            )
            if nc.is_connected:
                await nc.drain()
            if attempt < max_retries - 1:
                await asyncio.sleep(retry_delay)
                retry_delay = min(retry_delay * 2, 30)  # 指数退避
            else:
                logger.error("All NATS reconnection attempts exhausted, giving up")
                raise


if __name__ == '__main__':
    logging.basicConfig(level=logging.INFO)
    asyncio.run(run_consumer())
