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
        
        # Connect to Chroma host
        chroma_host = os.environ.get("CHROMA_HOST", "localhost")
        chroma_port = os.environ.get("CHROMA_PORT", "8000")
        
        hub_env = os.environ.get("HUB_ENV", "development")
        allow_reset = hub_env != "production"
        self.client = chromadb.HttpClient(
            host=chroma_host,
            port=chroma_port,
            settings=Settings(allow_reset=allow_reset)
        )
        
        # We use a collection per workspace for data isolation
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
        
        # Custom embedding function wrapper for chromadb
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
        """Chunk document and add to Chroma collection"""
        chunks = self.text_splitter.split_text(text_content)
        if not chunks:
            return 0
            
        ids = [f"{doc_id}_chunk_{i}" for i in range(len(chunks))]
        metadatas = [{"doc_id": doc_id, "title": title, "chunk_index": i} for i in range(len(chunks))]
        
        # Add to chroma
        self.collection.add(
            documents=chunks,
            metadatas=metadatas,
            ids=ids
        )
        return len(chunks)
        
    def search(self, query: str, top_k: int = 3) -> list:
        """Search the collection for relevant snippets"""
        # If no documents exist in collection, it returns empty
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
