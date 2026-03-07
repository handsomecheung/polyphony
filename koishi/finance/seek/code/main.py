#!/usr/bin/env python3.12

import os
import sys
from datetime import datetime
from typing import Optional, Union

import psycopg2
from psycopg2.extras import RealDictCursor
from fastapi import FastAPI, HTTPException, Query
from fastapi.responses import PlainTextResponse
import pytz

def get_required_env(key):
    value = os.environ.get(key)
    if not value:
        print(f"Error: Required environment variable '{key}' is not set or empty.")
        sys.exit(1)
    return value

DB_HOST = get_required_env("DB_HOST")
DB_PORT = get_required_env("DB_PORT")
DB_NAME = get_required_env("DB_NAME")
DB_USER = get_required_env("DB_USER")
DB_PASS = get_required_env("DB_PASS")

app = FastAPI(title="Finance Seek API")

def get_db_connection():
    return psycopg2.connect(
        host=DB_HOST,
        port=DB_PORT,
        dbname=DB_NAME,
        user=DB_USER,
        password=DB_PASS,
        cursor_factory=RealDictCursor
    )

@app.get("/ok")
def ok():
    return {"status": "ok"}


def response_price(symbol: str, record: Optional[dict], simple: bool) -> Union[PlainTextResponse, dict]:
    if not record:
        raise HTTPException(status_code=404, detail=f"No data found for symbol {symbol}")

    if simple:
        return PlainTextResponse(content=str(record["adj_close"]))
    return record


@app.get("/time")
def get_price_time(
    symbol: str = Query(..., description="The symbol to seek (e.g., GOOG, BTC-USD)"),
    time: str = Query(..., description="The time point (ISO format, e.g., 2024-03-06T12:00:00Z)"),
    simple: bool = Query(False, description="If true, return only the adjusted close price value as plain text")
):
    try:
        dt = datetime.fromisoformat(time.replace('Z', '+00:00'))
        if dt.tzinfo is None:
            dt = pytz.UTC.localize(dt)
        else:
            dt = dt.astimezone(pytz.UTC)
            
        day_start = dt.replace(hour=0, minute=0, second=0, microsecond=0)
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid time format. Use ISO format (e.g., YYYY-MM-DDTHH:MM:SSZ)")

    conn = None
    try:
        conn = get_db_connection()
        cur = conn.cursor()
        
        # the data is indexed by (symbol, time)
        cur.execute(
            "SELECT * FROM financial_prices WHERE symbol = %s AND time = %s",
            (symbol, day_start)
        )
        record = cur.fetchone()
        
        return response_price(symbol, record, simple)
    except HTTPException:
        raise
    except Exception as e:
        print(f"Database error: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")
    finally:
        if conn:
            conn.close()

@app.get("/latest")
def get_price_latest(
    symbol: str = Query(..., description="The symbol to seek (e.g., GOOG, BTC-USD)"),
    simple: bool = Query(False, description="If true, return only the adjusted close price value as plain text")
):
    conn = None
    try:
        conn = get_db_connection()
        cur = conn.cursor()
        
        cur.execute(
            "SELECT * FROM financial_prices WHERE symbol = %s ORDER BY time DESC LIMIT 1",
            (symbol,)
        )
        record = cur.fetchone()
        
        return response_price(symbol, record, simple)
    except HTTPException:
        raise
    except Exception as e:
        print(f"Database error: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")
    finally:
        if conn:
            conn.close()

if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000)
