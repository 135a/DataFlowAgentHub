import os
import logging
import chromadb
from chromadb.config import Settings
from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_openai import OpenAIEmbeddings

logger = logging.getLogger(__name__)

class KnowledgeBase:
    def __init__(self, workspace_id: str):
        self.workspace_id = workspace_id
        
        # 连接到 Chroma 主机
        chroma_host = os.environ.get("CHROMA_HOST", "localhost")
        chroma_port = os.environ.get("CHROMA_PORT", "8000")
        
        hub_env = os.environ.get("HUB_ENV", "development")
        allow_reset = hub_env != "production"
        self.client = chromadb.HttpClient(
            host=chroma_host,
            port=chroma_port,
            settings=Settings(allow_reset=allow_reset)
        )
        
        # 每个工作区使用独立的集合以实现数据隔离
        collection_name = f"workspace_{workspace_id.replace('-', '_')}"
        
        api_key = os.environ.get("OPENAI_API_KEY")
        base_url = os.environ.get("OPENAI_BASE_URL", "https://api.openai.com/v1")
        if not api_key:
            logger.warning("No OPENAI_API_KEY provided, RAG embeddings may fail if called")
            
        self.embeddings = OpenAIEmbeddings(
            api_key=api_key or "dummy",
            base_url=base_url,
            model="text-embedding-3-small"
        )
        
        # 自定义 chromadb 嵌入函数包装器
        class LangchainEmbeddingFunc:
            def __init__(self, embs):
                self.embs = embs
            def __call__(self, input):
                return self.embs.embed_documents(input)
                
        self.collection = self.client.get_or_create_collection(
            name=collection_name,
            embedding_function=LangchainEmbeddingFunc(self.embeddings)
        )
        
        self.text_splitter = RecursiveCharacterTextSplitter(
            chunk_size=1000,
            chunk_overlap=200,
            length_function=len,
        )

    def add_document(self, doc_id: str, title: str, text_content: str):
        """将文档分块并添加到 Chroma 集合"""
        chunks = self.text_splitter.split_text(text_content)
        if not chunks:
            return 0
            
        ids = [f"{doc_id}_chunk_{i}" for i in range(len(chunks))]
        metadatas = [{"doc_id": doc_id, "title": title, "chunk_index": i} for i in range(len(chunks))]
        
        # 添加到 Chroma
        self.collection.add(
            documents=chunks,
            metadatas=metadatas,
            ids=ids
        )
        return len(chunks)
        
    def search(self, query: str, top_k: int = 3) -> list:
        """在集合中搜索相关片段"""
        # 如果集合中没有文档，返回空列表
        if self.collection.count() == 0:
            return []
            
        results = self.collection.query(
            query_texts=[query],
            n_results=top_k
        )
        
        snippets = []
        if not results.get('documents') or not results['documents'][0]:
            return snippets
            
        docs = results['documents'][0]
        metas = results['metadatas'][0]
        distances = results['distances'][0] if 'distances' in results and results['distances'] else [0.0] * len(docs)
        
        for doc, meta, dist in zip(docs, metas, distances):
            snippets.append({
                "content": doc,
                "metadata": meta,
                "distance": dist
            })
            
        return snippets
