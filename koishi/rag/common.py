import os
import sys
import redis
from openai import OpenAI

REQUIRED_VARS = [
    "REDIS_HOST",
    "OLLAMA_API_BASE",
    "LITELLM_API_BASE",
    "NOTES_PATH"
]

def check_env():
    missing = [var for var in REQUIRED_VARS if not os.getenv(var)]
    if missing:
        print(f"Error: Missing required environment variables: {', '.join(missing)}")
        print("Please set them before running the script.")
        sys.exit(1)

check_env()

REDIS_HOST = os.getenv("REDIS_HOST")
REDIS_PORT = int(os.getenv("REDIS_PORT", 6379))
REDIS_DB = int(os.getenv("REDIS_DB", 0))
INDEX_NAME = os.getenv("INDEX_NAME", "obsidian_notes")

EMBEDDING_MODEL = os.getenv("EMBEDDING_MODEL", "bge-m3")
LLM_MODEL = os.getenv("LLM_MODEL", "ollama-local")

OLLAMA_API_BASE = os.getenv("OLLAMA_API_BASE")
LITELLM_API_BASE = os.getenv("LITELLM_API_BASE")
LITELLM_API_KEY = os.getenv("LITELLM_API_KEY", "any-key")

NOTES_PATH = os.getenv("NOTES_PATH")

chat_client = OpenAI(api_key=LITELLM_API_KEY, base_url=LITELLM_API_BASE)
redis_client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, db=REDIS_DB, decode_responses=True)

_embed_client = OpenAI(api_key="ollama", base_url=OLLAMA_API_BASE)

def call_embedding(query_text):
    return _embed_client.embeddings.create(input=[query_text], model=EMBEDDING_MODEL)
