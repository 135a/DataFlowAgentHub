import os
import time
import requests
import json

def test_rag_upload_and_search():
    base_url = "http://localhost:8080/api/v1"
    
    # 1. Login to get token and workspace_id
    resp = requests.post(f"http://localhost:8080/v1/auth/login", json={
        "email": "admin@demo.local",
        "password": "changeme"
    })
    
    if resp.status_code != 200:
        print("Login failed:", resp.text)
        return
        
    data = resp.json()
    token = data["access_token"]
    ws_id = data["workspace_id"]
    
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }
    
    # 2. Upload document
    print("Uploading RAG document...")
    doc_resp = requests.post(f"{base_url}/workspaces/{ws_id}/knowledge/docs", headers=headers, json={
        "title": "业务指标口径字典",
        "content": "核心有效活跃用户 (Core MAU): 指在一个自然月内，登录过系统且执行了至少一次核心操作（如发起查询、创建报表）的用户。"
    })
    
    print("Upload response:", doc_resp.status_code, doc_resp.text)
    
    if doc_resp.status_code != 202:
        return
        
    print("Wait for async indexing...")
    time.sleep(3) # simulate waiting for worker
    
    # We can list docs to see it
    list_resp = requests.get(f"{base_url}/workspaces/{ws_id}/knowledge/docs", headers=headers)
    print("Docs:", json.dumps(list_resp.json(), indent=2))
    
    print("Done (Note: Search testing requires the Python worker to actually index and we can test it directly or via Agent).")

if __name__ == "__main__":
    test_rag_upload_and_search()
