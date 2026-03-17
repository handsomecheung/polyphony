import sys
import numpy as np
from redis.commands.search.query import Query
from common import redis_client, chat_client, INDEX_NAME, LLM_MODEL, call_embedding

def search_notes(query_text, top_k=3):
    response = call_embedding(query_text)
    query_vector = np.array(response.data[0].embedding, dtype=np.float32).tobytes()
    
    q = (
        Query(f"(*)=>[KNN {top_k} @vector $vec as score]")
        .sort_by("score")
        .return_fields("content", "path", "score")
        .dialect(2)
    )
    
    results = redis_client.ft(INDEX_NAME).search(q, query_params={"vec": query_vector})
    return results.docs

def rag_query(query_text):
    relevant_docs = search_notes(query_text)
    if not relevant_docs:
        return "No relevant notes found."
        
    context = "\n---\n".join([doc.content for doc in relevant_docs])
    
    prompt = f"""Use the following pieces of context to answer the user's question. 
If you don't know the answer based on the context, just say you don't know.

Context:
{context}

Question: {query_text}
Answer:"""

    response = chat_client.chat.completions.create(
        model=LLM_MODEL,
        messages=[{"role": "user", "content": prompt}]
    )
    return response.choices[0].message.content

if __name__ == "__main__":
    if len(sys.argv) > 1:
        print(rag_query(" ".join(sys.argv[1:])))
    else:
        print("Usage: pipenv run python search.py \"your question\"")
