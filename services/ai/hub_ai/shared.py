import hashlib
import hmac


def sign_body(secret: str, body: bytes) -> str:
    """Return X-Hub-Signature header value for the request body."""
    mac = hmac.new(secret.encode(), body, hashlib.sha256)
    return f"sha256={mac.hexdigest()}"


def make_headers(secret: str, body_bytes: bytes) -> dict:
    """Build headers with HMAC signature for internal API calls."""
    return {
        "X-Hub-Signature": sign_body(secret, body_bytes),
        "Content-Type": "application/json",
    }
