import hashlib
import hmac


def sign_body(secret: str, body: bytes) -> str:
    """返回请求体的 X-Hub-Signature 头部值。"""
    mac = hmac.new(secret.encode(), body, hashlib.sha256)
    return f"sha256={mac.hexdigest()}"


def make_headers(secret: str, body_bytes: bytes) -> dict:
    """构建带有 HMAC 签名的内部 API 调用头部。"""
    return {
        "X-Hub-Signature": sign_body(secret, body_bytes),
        "Content-Type": "application/json",
    }
