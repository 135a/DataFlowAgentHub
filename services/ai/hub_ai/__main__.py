import json
import logging
import os
import re
import sys
from concurrent import futures

import grpc

_WORKER_VERSION = "0.1.0"


def _setup_logging():
    logging.basicConfig(
        level=os.environ.get("LOG_LEVEL", "INFO"),
        format='{"level":"%(levelname)s","msg":"%(message)s","logger":"%(name)s"}',
    )


def _read_only_ok(sql: str) -> tuple[bool, str]:
    """不再拦截写操作 — 权限交由 Go 侧 sqlrun 执行器统一管理。
    保留此函数只做基本格式检查，不做关键字拦截。"""
    s = sql.strip().rstrip(";").upper()
    if not s:
        return False, "empty sql"
    return True, ""


def main():
    _setup_logging()
    base = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "gen"))
    if base not in sys.path:
        sys.path.insert(0, base)
    try:
        from nl2sql.v1 import nl2sql_pb2, nl2sql_pb2_grpc
    except ImportError:
        logging.error("missing generated stubs; run `make gen-py` or build Docker image")
        sys.exit(1)

    class Servicer(nl2sql_pb2_grpc.NL2SQLServiceServicer):
        def Health(self, request, context):
            return nl2sql_pb2.HealthResponse(version=_WORKER_VERSION, ok=True)

        def GenerateSQL(self, request, context):
            md = dict(context.invocation_metadata())
            trace = request.trace_id or md.get("x-trace-id") or ""
            logging.getLogger("nl2sql").info(
                json.dumps(
                    {
                        "event": "generate_sql",
                        "trace_id": trace,
                        "session_id": request.session_id,
                    }
                )
            )
            msg = request.user_message or ""
            if os.environ.get("OPENAI_API_KEY"):
                sql, notes = self._openai_sql(request)
            else:
                sql = "SELECT 1 AS ok"
                notes = "fallback (no OPENAI_API_KEY)"
                m = re.search(r"\d+", msg)
                if m:
                    sql = f"SELECT {m.group()} AS n"
            # 模型返回 ERROR: 前缀表示需要用户补充信息，不是真正的 SQL
            if sql.startswith("ERROR:"):
                return nl2sql_pb2.GenerateSQLResponse(
                    ok=False, error_message=sql[6:].strip(), sql="", self_check_notes=""
                )
            return nl2sql_pb2.GenerateSQLResponse(
                ok=True, sql=sql, self_check_notes=notes, error_message=""
            )

        def RunAgentPipeline(self, request, context):
            md = dict(context.invocation_metadata())
            trace = request.trace_id or md.get("x-trace-id") or ""
            logging.getLogger("agent_pipeline").info(
                json.dumps({
                    "event": "run_agent_pipeline",
                    "trace_id": trace,
                    "session_id": request.session_id,
                    "run_id": request.run_id,
                })
            )
            
            try:
                from orchestrator.graph import workflow_graph
                initial_state = {
                    "run_id": request.run_id,
                    "user_input": request.user_message,
                    "schema_context": request.schema_json,
                }
                config = {"configurable": {"thread_id": request.session_id}}
                result = workflow_graph.invoke(initial_state, config=config)
                # Note: sync invoke in gRPC handler; Go side has its own deadline
                
                return nl2sql_pb2.RunAgentPipelineResponse(
                    ok=True, 
                    error_message="", 
                    final_report=result.get("final_report", "")
                )
            except Exception as e:
                logging.error(f"Pipeline failed: {e}")
                return nl2sql_pb2.RunAgentPipelineResponse(
                    ok=False, 
                    error_message=str(e), 
                    final_report=""
                )

        def _openai_sql(self, request):
            from openai import OpenAI

            client = OpenAI(
                api_key=os.environ["OPENAI_API_KEY"],
                base_url=os.environ.get("OPENAI_BASE_URL", "https://api.openai.com/v1"),
            )
            model = os.environ.get("OPENAI_MODEL", "gpt-4o-mini")
            schema_text = self._format_schema(request.schema_json or "{}")
            prompt = (
                "You are a Postgres SQL generator. Reply ONLY with SQL, no markdown.\n"
                f"Tables:\n{schema_text}\n"
                f"Question: {request.user_message}\n"
                "Rules:\n"
                "- SELECT for queries;\n"
                "- INSERT/UPDATE/DELETE for data changes;\n"
                "- CREATE TABLE for new tables;\n"
                "- CRITICAL: NEVER fabricate data. If user asks to insert/update but "
                "does NOT provide specific values, reply with 'ERROR: ' followed by a "
                "Chinese question asking what data they want to provide. "
                "Example: user says '添加一条客户记录' → "
                "ERROR: 请提供要添加的客户信息，例如姓名、联系方式等\n"
                "- Only generate INSERT/UPDATE when user provides actual values;\n"
                "- NEVER touch system tables (users, workspaces, sessions, etc.);\n"
                "- Use public schema if unspecified."
            )
            r = client.chat.completions.create(
                model=model,
                messages=[{"role": "user", "content": prompt}],
                temperature=0.1,
                timeout=60.0,
            )
            sql = (r.choices[0].message.content or "").strip()
            for prefix in ("```sql", "```"):
                if sql.startswith(prefix):
                    sql = sql[len(prefix) :].strip()
            if sql.endswith("```"):
                sql = sql[:-3].strip()
            return sql, "openai"

        def _format_schema(self, schema_json: str) -> str:
            """将 schema JSON 转换为可读的表描述信息（带截断）。"""
            import json
            MAX_CHARS = 6000
            try:
                schema = json.loads(schema_json)
            except (json.JSONDecodeError, TypeError):
                return "(no schema available)"

            tables = schema.get("tables", [])
            if not tables:
                return "(no tables discovered)"

            lines = []
            total_chars = 0
            truncated = False
            for table in tables:
                name = table.get("name", "unknown")
                cols = table.get("columns", [])
                col_parts = []
                for col in cols:
                    cname = col.get("name", "?")
                    ctype = col.get("type", "text")
                    col_parts.append(f"{cname} ({ctype})")
                line = f"- {name}: " + ", ".join(col_parts)
                total_chars += len(line) + 1
                if total_chars > MAX_CHARS and lines:
                    truncated = True
                    break
                lines.append(line)

            result = "\n".join(lines)
            if truncated:
                remaining = len(tables) - len(lines)
                result += f"\n(Schema truncated, {remaining} table(s) omitted)"
            return result

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    nl2sql_pb2_grpc.add_NL2SQLServiceServicer_to_server(Servicer(), server)
    addr = os.environ.get("WORKER_GRPC_ADDR", "0.0.0.0:50051")

    # mTLS 支持：若配置了证书则使用安全端口，否则回退到 insecure
    ca_cert = os.environ.get("HUB_GRPC_CA_CERT") or ""
    server_cert = os.environ.get("HUB_GRPC_SERVER_CERT") or ""
    server_key = os.environ.get("HUB_GRPC_SERVER_KEY") or ""
    if ca_cert and server_cert and server_key:
        with open(server_cert, "rb") as f:
            server_cert_bytes = f.read()
        with open(server_key, "rb") as f:
            server_key_bytes = f.read()
        with open(ca_cert, "rb") as f:
            ca_bytes = f.read()
        creds = grpc.ssl_server_credentials(
            [(server_key_bytes, server_cert_bytes)],
            root_certificates=ca_bytes,
            require_client_auth=True,
        )
        server.add_secure_port(addr, creds)
        logging.getLogger(__name__).info("grpc mTLS enabled on %s", addr)
    else:
        server.add_insecure_port(addr)
        logging.getLogger(__name__).info("grpc insecure on %s", addr)

    server.start()
    logging.getLogger(__name__).info("grpc listening on %s", addr)
    
    # 9.3 Start NATS consumers in background
    def start_agent_consumer():
        import asyncio
        from orchestrator.consumer import run_consumer
        try:
            asyncio.run(run_consumer())
        except Exception as e:
            logging.error(f"Agent consumer died: {e}")

    def start_knowledge_consumer():
        import asyncio
        from orchestrator.knowledge_consumer import run_knowledge_consumer
        try:
            asyncio.run(run_knowledge_consumer())
        except Exception as e:
            logging.error(f"Knowledge consumer died: {e}")

    import threading
    agent_thread = threading.Thread(target=start_agent_consumer, daemon=True)
    agent_thread.start()
    knowledge_thread = threading.Thread(target=start_knowledge_consumer, daemon=True)
    knowledge_thread.start()
    logging.getLogger(__name__).info("Started NATS consumer threads (agent + knowledge)")
    
    server.wait_for_termination()

if __name__ == "__main__":
    main()
