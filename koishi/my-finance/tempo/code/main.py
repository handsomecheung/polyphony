#!/usr/bin/env python3.12

import os
import sys
import time

import psycopg2
from psycopg2.extras import execute_values
from datetime import datetime, timedelta, date
import pytz
import pandas as pd
import yfinance as yf

SYMBOLS = {
    "GOOG": "GOOG",
    "SPY": "SPY",
    "VGT": "VGT",
    "BTC-USD": "BTC-USD",
    "ETH-USD": "ETH-USD",
    "ADA-USD": "ADA-USD"
}

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

def get_db_connection():
    return psycopg2.connect(
        host=DB_HOST,
        port=DB_PORT,
        dbname=DB_NAME,
        user=DB_USER,
        password=DB_PASS
    )

def init_db():
    conn = get_db_connection()
    cur = conn.cursor()
    
    cur.execute("""
        CREATE TABLE IF NOT EXISTS financial_prices (
            time TIMESTAMPTZ NOT NULL,
            symbol TEXT NOT NULL,
            open DOUBLE PRECISION,
            high DOUBLE PRECISION,
            low DOUBLE PRECISION,
            close DOUBLE PRECISION,
            adj_close DOUBLE PRECISION,
            volume BIGINT,
            PRIMARY KEY (symbol, time)
        );
    """)
    
    conn.commit()
    cur.close()
    conn.close()

def fetch_and_store(symbol_key, ticker_symbol):
    print(f"Processing {symbol_key} ({ticker_symbol})...")
    conn = get_db_connection()
    cur = conn.cursor()
    
    cur.execute("SELECT MAX(time) FROM financial_prices WHERE symbol = %s", (symbol_key,))
    latest_db_time = cur.fetchone()[0]
    
    if latest_db_time is None:
        print(f"No existing data for {symbol_key}. Fetching maximum available history.")
        df = yf.download(ticker_symbol, period="max", interval='1d')
    else:
        yesterday = date.today() - timedelta(days=1)
        if latest_db_time.date() < yesterday:
            print(f"Gap detected for {symbol_key}. Latest: {latest_db_time.date()}, Yesterday: {yesterday}")
        
        start_date = (latest_db_time + timedelta(days=1)).strftime('%Y-%m-%d')
        print(f"Fetching updates for {symbol_key} from {start_date}")
        df = yf.download(ticker_symbol, start=start_date, interval='1d')

    if df.empty:
        print(f"No new data found for {symbol_key}.")
        cur.close()
        conn.close()
        return

    # Prepare data for insertion
    # yfinance multi-index handling (newer versions return multi-index even for single ticker if not careful)
    if isinstance(df.columns, pd.MultiIndex):
        df.columns = df.columns.get_level_values(0)

    data_to_insert = []
    for timestamp, row in df.iterrows():
        # Ensure timestamp is UTC and normalized to midnight
        # yf daily data usually comes as 00:00:00 local or UTC
        dt = timestamp.to_pydatetime()
        if dt.tzinfo is None:
            dt = pytz.UTC.localize(dt)
        else:
            dt = dt.astimezone(pytz.UTC)
            
        # Normalize to midnight if it isn't already (yf daily usually is)
        dt = dt.replace(hour=0, minute=0, second=0, microsecond=0)
        
        data_to_insert.append((
            dt,
            symbol_key,
            float(row['Open']),
            float(row['High']),
            float(row['Low']),
            float(row['Close']),
            float(row['Adj Close']) if 'Adj Close' in row else float(row['Close']),
            int(row['Volume']) if not pd.isna(row['Volume']) else 0
        ))

    insert_query = """
        INSERT INTO financial_prices (time, symbol, open, high, low, close, adj_close, volume)
        VALUES %s
        ON CONFLICT (symbol, time) DO UPDATE SET
            open = EXCLUDED.open,
            high = EXCLUDED.high,
            low = EXCLUDED.low,
            close = EXCLUDED.close,
            adj_close = EXCLUDED.adj_close,
            volume = EXCLUDED.volume;
    """
    
    try:
        execute_values(cur, insert_query, data_to_insert)
        conn.commit()
        print(f"Successfully updated {len(data_to_insert)} records for {symbol_key}.")
    except Exception as e:
        conn.rollback()
        print(f"Error updating {symbol_key}: {e}")
    finally:
        cur.close()
        conn.close()

def main():
    print("Starting financial data fetcher...")
    init_db()
    
    for symbol_key, ticker_symbol in SYMBOLS.items():
        try:
            fetch_and_store(symbol_key, ticker_symbol)
        except Exception as e:
            print(f"Failed to process {symbol_key}: {e}")
        time.sleep(1)

if __name__ == "__main__":
    main()
