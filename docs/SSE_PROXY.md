# SSE 反向代理（Nginx / Caddy）

浏览器订阅 `GET /v1/sessions/{id}/stream` 使用 **Server-Sent Events**。

- `user_message`: Server echoed user message.
- `run_started`: `{"run_id":"..."}`
- `agent_step`: `{"agent_name": "...", "status": "...", "summary": "...", "error": "..."}` - Fired when a LangGraph node executes.
- `sql_generated`: `{"sql":"SELECT ..."}`
- `result`: The final AI response, either `{"sql":"...","rows":[...]}` or `{"error":"..."}`.
- `approval_required`: `{"run_id":"..."}` - A prompt to export data needing human review.
- `approval_approved` / `approval_rejected`
- `error`: Generic error structure.

反向代理默认可能缓冲响应，导致事件延迟。

## Nginx

```nginx
location /v1/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_buffering off;
    proxy_read_timeout 3600s;
    add_header X-Accel-Buffering no;
}
```

## Caddy

```caddy
handle /v1/* {
    reverse_proxy localhost:8080 {
        flush_interval -1
    }
}
```

应用已对 SSE 响应设置 `X-Accel-Buffering: no`，与 Nginx 配置配合使用。
