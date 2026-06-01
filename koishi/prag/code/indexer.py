import glob
import os
import numpy as np
import time
from langchain_text_splitters import RecursiveCharacterTextSplitter
from redis.commands.search.field import VectorField, TextField, TagField
from redis.commands.search.index_definition import IndexDefinition, IndexType
from redis.commands.search.query import Query
from common import redis_client, INDEX_NAME, NOTES_PATH, call_embedding

REGISTRY_KEY = "prag:registry:mtime"
BATCH_SIZE = 32  # Number of chunks to embed and store in one batch

def get_rel_path(abs_path):
    """Returns the path relative to NOTES_PATH."""
    return os.path.relpath(abs_path, NOTES_PATH)

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
            definition=IndexDefinition(prefix=["prag:default:"], index_type=IndexType.HASH)
        )
        print(f"Created index {INDEX_NAME}.")

def delete_file_chunks(rel_path):
    """
    Deletes all chunks associated with a specific relative file path from the index.
    """
    # Escaping special characters for TagField query
    escaped_path = rel_path.replace("-", "\\-").replace(".", "\\.").replace("_", "\\_").replace("/", "\\/")
    query_str = f"@path:{{{escaped_path}}}"
    
    while True:
        query = Query(query_str).return_fields("id").paging(0, 1000).dialect(2)
        results = redis_client.ft(INDEX_NAME).search(query)
        if not results.docs:
            break
        
        ids = [doc.id for doc in results.docs]
        redis_client.delete(*ids)
        if len(results.docs) < 1000:
            break

def process_file_in_batches(abs_path, splitter):
    """
    Reads a file and yields batches of chunks.
    """
    try:
        with open(abs_path, 'r', encoding='utf-8') as f:
            content = f.read()
            chunks = splitter.split_text(content)
            
            for i in range(0, len(chunks), BATCH_SIZE):
                yield chunks[i:i + BATCH_SIZE], i
    except Exception as e:
        print(f"Error reading {abs_path}: {e}")
        return

def cleanup_deleted_files(all_rel_paths_on_disk):
    """
    Removes files from the index and registry that no longer exist on disk.
    Uses relative paths.
    """
    registry = redis_client.hgetall(REGISTRY_KEY)
    if not registry:
        return
        
    all_files_set = set(all_rel_paths_on_disk)
    for rel_path in registry:
        if rel_path not in all_files_set:
            print(f"File deleted on disk, removing from index: {rel_path}")
            delete_file_chunks(rel_path)
            redis_client.hdel(REGISTRY_KEY, rel_path)

def index_notes():
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=1000,
        chunk_overlap=100,
        separators=["\n## ", "\n# ", "\n\n", "\n", " ", ""]
    )
    
    # Ensure NOTES_PATH is absolute and exists
    abs_notes_path = os.path.abspath(NOTES_PATH)
    all_abs_files = glob.glob(f"{abs_notes_path}/**/*.md", recursive=True)
    
    # Convert all found files to relative paths for comparison and registry
    all_rel_files = [os.path.relpath(f, abs_notes_path) for f in all_abs_files]
    print(f"Found {len(all_rel_files)} markdown files.")
    
    # Cleanup files that are no longer on disk using relative paths
    cleanup_deleted_files(all_rel_files)
    
    for rel_path, abs_path in zip(all_rel_files, all_abs_files):
        mtime = os.path.getmtime(abs_path)
        old_mtime = redis_client.hget(REGISTRY_KEY, rel_path)
        
        if old_mtime and float(old_mtime) == mtime:
            continue
        
        print(f"Indexing: {rel_path}")
        # Clear existing index using relative path
        delete_file_chunks(rel_path)
        
        for batch_chunks, start_idx in process_file_in_batches(abs_path, splitter):
            if not batch_chunks:
                continue
                
            # Batch embedding
            response = call_embedding(batch_chunks)
            embeddings = [np.array(emb.embedding, dtype=np.float32).tobytes() for emb in response.data]
            
            # Batch storage using relative path
            pipeline = redis_client.pipeline()
            for i, (chunk, vector) in enumerate(zip(batch_chunks, embeddings)):
                # doc_id also uses relative path to be consistent
                doc_id = f"prag:default:{rel_path}:{start_idx + i}"
                pipeline.hset(doc_id, mapping={
                    "content": chunk,
                    "path": rel_path,
                    "vector": vector
                })
            pipeline.execute()
            
        # Update registry with relative path
        redis_client.hset(REGISTRY_KEY, rel_path, mtime)
        print(f"Finished: {rel_path}")

if __name__ == "__main__":
    start_time = time.time()
    create_index()
    index_notes()
    print(f"Indexing completed in {time.time() - start_time:.2f} seconds.")
