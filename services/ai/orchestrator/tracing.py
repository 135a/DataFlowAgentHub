import os
import json
import hmac
import hashlib
import httpx
import logging

logger = logging.getLogger(__name__)


def sign_body(secret: str, body: bytes) -> str:
    """Return X-Hub-Signature header value for the request body."""
    mac = hmac.new(secret.encode(), body, hashlib.sha256)
    return f"sha256={mac.hexdigest()}"


def report_run_step(run_id: str, agent_name: str, status: str, input_summary: str = "", output_summary: str = "", error_message: str = ""):
    api_url = os.environ.get("HUB_API_INTERNAL_URL", "http://api:8080")
    secret = os.environ.get("HUB_INTERNAL_HMAC_SECRET", "dev-hmac-secret-change-me")

    payload = {
        "agent_name": agent_name,
        "status": status,
        "input_summary": input_summary[:1000],
        "output_summary": output_summary[:1000],
        "error_message": error_message[:1000]
    }

    body_bytes = json.dumps(payload).encode()
    headers = {
        "X-Hub-Signature": sign_body(secret, body_bytes),
        "Content-Type": "application/json",
    }

    try:
        httpx.post(f"{api_url}/internal/runs/{run_id}/steps", headers=headers, content=body_bytes, timeout=2.0)
    except Exception as e:
        logger.warning(f"Failed to report run step to Go API: {e}")
