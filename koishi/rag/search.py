import sys
import json
import numpy as np
from redis.commands.search.query import Query
from common import redis_client, INDEX_NAME, call_embedding

def search_notes(query_text, top_k=5):
    """
    Retrieves relevant notes from Redis and returns them as a list of dicts.
    """
    response = call_embedding(query_text)
    query_vector = np.array(response.data[0].embedding, dtype=np.float32).tobytes()
    
    # Redis KNN using COSINE distance returns distance (0-1).
    # Similarity = 1 - distance
    q = (
        Query(f"(*)=>[KNN {top_k} @vector $vec as dist]")
        .sort_by("dist")
        .return_fields("content", "path", "dist")
        .dialect(2)
    )
    
    results = redis_client.ft(INDEX_NAME).search(q, query_params={"vec": query_vector})
    
    docs = []
    for doc in results.docs:
        # Convert distance to similarity score
        similarity = 1 - float(doc.dist)
        docs.append({
            "content": doc.content,
            "score": round(similarity, 4),
            "source": doc.path
        })
    
    return docs

if __name__ == "__main__":
    if len(sys.argv) > 1:
        query = " ".join(sys.argv[1:])
        results = search_notes(query)
        print(json.dumps(results, ensure_ascii=False, indent=2))
    else:
        print("Usage: pipenv run python search.py \"your query\"")
