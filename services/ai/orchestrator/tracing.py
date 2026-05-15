import os
import httpx
import logging

logger = logging.getLogger(__name__)

def report_run_step(run_id: str, agent_name: str, status: str, input_summary: str = "", output_summary: str = "", error_message: str = ""):
    api_url = os.environ.get("HUB_API_INTERNAL_URL", "http://api:8080")
    secret = os.environ.get("HUB_INTERNAL_HMAC_SECRET", "dev-hmac-secret-change-me")
    
    # In a real system, compute HMAC for auth here.
    # For MVP, we'll just pass a simple header or internal API key
    headers = {
        "X-Hub-Internal-Secret": secret,
        "Content-Type": "application/json"
    }
    
    payload = {
        "agent_name": agent_name,
        "status": status,
        "input_summary": input_summary[:1000],
        "output_summary": output_summary[:1000],
        "error_message": error_message[:1000]
    }
    
    try:
        httpx.post(f"{api_url}/internal/runs/{run_id}/steps", headers=headers, json=payload, timeout=2.0)
    except Exception as e:
        logger.warning(f"Failed to report run step to Go API: {e}")
