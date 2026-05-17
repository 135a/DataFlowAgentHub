import logging
from hub_ai.internal_client import HubInternalClient

logger = logging.getLogger(__name__)

# 全局 gRPC 客户端（惰性初始化）
_internal_client: HubInternalClient | None = None


def _get_internal_client() -> HubInternalClient:
    global _internal_client
    if _internal_client is None:
        _internal_client = HubInternalClient()
    return _internal_client


def report_run_step(run_id: str, agent_name: str, status: str, input_summary: str = "", output_summary: str = "", error_message: str = ""):
    try:
        client = _get_internal_client()
        client.run_step_callback(
            run_id=run_id,
            agent_name=agent_name,
            status=status,
            input_summary=input_summary,
            output_summary=output_summary,
            error_message=error_message,
        )
    except Exception as e:
        logger.warning(f"Failed to report run step via gRPC: {e}")
