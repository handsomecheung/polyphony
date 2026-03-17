import glob
import numpy as np
from langchain_text_splitters import RecursiveCharacterTextSplitter
from redis.commands.search.field import VectorField, TextField, TagField
from redis.commands.search.index_definition import IndexDefinition, IndexType
from common import redis_client, INDEX_NAME, NOTES_PATH, call_embedding

def create_index(dim=1024):
    try:
        redis_client.ft(INDEX_NAME).info()
        print(f"Index {INDEX_NAME} already exists.")
    except Exception as e:
        print(f"Index not found or error: {e}. Creating new index...")
        schema = (
            TextField("content"),
            TagField("path"),
            VectorField("vector", "HNSW", {
                "TYPE": "FLOAT32",
                "DIM": dim,
                "DISTANCE_METRIC": "COSINE"
            })
        )
        redis_client.ft(INDEX_NAME).create_index(
            schema, 
            definition=IndexDefinition(prefix=["rag:obsidian:"], index_type=IndexType.HASH)
        )
        print(f"Created index {INDEX_NAME}.")

def get_embedding(text):
    response = call_embedding(text)
    return np.array(response.data[0].embedding, dtype=np.float32).tobytes()

def index_notes():
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=500, chunk_overlap=50, separators=["\n\n", "\n", " ", ""]
    )
    files = glob.glob(f"{NOTES_PATH}/**/*.md", recursive=True)
    print(f"Found {len(files)} markdown files.")
    
    for file_path in files:
        with open(file_path, 'r', encoding='utf-8') as f:
            text = f.read()
            chunks = splitter.split_text(text)
            for i, chunk in enumerate(chunks):
                doc_id = f"rag:obsidian:{file_path}:{i}"
                redis_client.hset(doc_id, mapping={
                    "content": chunk,
                    "path": file_path,
                    "vector": get_embedding(chunk)
                })
        print(f"Indexed: {file_path}")

if __name__ == "__main__":
    create_index()
    index_notes()
