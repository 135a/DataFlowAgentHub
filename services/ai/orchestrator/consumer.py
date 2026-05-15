import asyncio
import json
import logging
import os
import httpx
from nats.aio.client import Client as NATS
from orchestrator.graph import workflow_graph

logger = logging.getLogger(__name__)

async def process_message(msg):
    try:
        data = json.loads(msg.data.decode())
        task_id = data.get("id")
        session_id = data.get("session_id", "")
        run_id = data.get("run_id", "")
        payload = data.get("payload", {})
        
        logger.info(f"Processing task {task_id} (run_id: {run_id})")
        
        # Build initial state
        initial_state = {
            "run_id": run_id,
            "user_input": payload.get("user_message", ""),
            "schema_context": payload.get("schema_json", "{}"),
        }
        config = {"configurable": {"thread_id": session_id or task_id}}
        
        # Run graph synchronously inside async wrapper for MVP
        # In a real system, we'd use workflow_graph.ainvoke
        result = await asyncio.to_thread(workflow_graph.invoke, initial_state, config)
        
        # Callback to Go API
        api_url = os.environ.get("HUB_API_INTERNAL_URL", "http://api:8080")
        secret = os.environ.get("HUB_INTERNAL_HMAC_SECRET", "dev-hmac-secret-change-me")
        
        headers = {
            "X-Hub-Internal-Secret": secret,
            "Content-Type": "application/json"
        }
        
        callback_payload = {
            "status": "succeeded",
            "result": {
                "final_report": result.get("final_report", ""),
                "analysis_summary": result.get("analysis_summary", "")
            },
            "error_message": ""
        }
        
        async with httpx.AsyncClient() as client:
            resp = await client.post(
                f"{api_url}/internal/tasks/{task_id}/callback",
                headers=headers,
                json=callback_payload,
                timeout=5.0
            )
            resp.raise_for_status()
            logger.info(f"Task {task_id} callback successful")
            
    except Exception as e:
        logger.error(f"Task processing failed: {e}", exc_info=True)
        try:
            callback_payload = {
                "status": "failed",
                "result": {},
                "error_message": str(e)
            }
            async with httpx.AsyncClient() as client:
                await client.post(
                    f"{api_url}/internal/tasks/{task_id}/callback",
                    headers=headers,
                    json=callback_payload,
                    timeout=5.0
                )
        except Exception as cb_err:
            logger.error(f"Failed to send failure callback: {cb_err}")

async def run_consumer():
    nc = NATS()
    nats_url = os.environ.get("NATS_URL", "nats://localhost:4222")
    
    try:
        await nc.connect(nats_url)
        logger.info(f"Connected to NATS at {nats_url}")
        
        # Subscribe to agent pipeline topic
        await nc.subscribe("hub.tasks.agent_pipeline", cb=process_message)
        
        # Keep running
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
