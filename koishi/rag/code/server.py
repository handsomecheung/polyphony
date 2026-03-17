import os
import asyncio
from contextlib import asynccontextmanager
from fastapi import FastAPI, Query
from apscheduler.schedulers.background import BackgroundScheduler
from indexer import create_index, index_notes
from search import search_notes
from common import redis_client, INDEX_NAME

def check_and_reindex():
    """
    Check if the index is empty or missing, and trigger indexing if needed.
    """
    create_index()
    
    try:
        info = redis_client.ft(INDEX_NAME).info()
        num_docs = int(info.get('num_docs', 0))
        if num_docs == 0:
            print("Index is empty. Triggering full re-indexing...")
            index_notes()
    except Exception as e:
        print(f"Error checking index status: {e}. Indexing might be needed.")
        index_notes()

@asynccontextmanager
async def lifespan(app: FastAPI):
    scheduler = BackgroundScheduler()
    scheduler.add_job(index_notes, 'interval', hours=1, id='index_job')
    scheduler.add_job(check_and_reindex, 'interval', minutes=5, id='check_empty_job')
    scheduler.start()
    
    # Run initial index on startup in a separate thread/task to avoid blocking
    # We use loop.run_in_executor for CPU-heavy indexing if needed, 
    # but index_notes is synchronous and simple enough for now.
    asyncio.create_task(asyncio.to_thread(check_and_reindex))
    
    yield
    scheduler.shutdown()

app = FastAPI(title="Obsidian RAG API", lifespan=lifespan)

@app.get("/query")
def query_rag(q: str = Query(..., description="The query string"), top_k: int = 5):
    """
    Search for relevant notes based on the query string.
    """
    results = search_notes(q, top_k=top_k)
    return results

@app.get("/ok")
def ok():
    return {"status": "ok"}

@app.post("/index")
def trigger_index():
    """
    Manually trigger an indexing process.
    """
    index_notes()
    return {"message": "Indexing started"}

if __name__ == "__main__":
    import uvicorn
    create_index()
    uvicorn.run(app, host="0.0.0.0", port=8000)
