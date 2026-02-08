#!/usr/bin/env python3
"""
Script to load 1 year of historical Nifty 50 stock data into MySQL database.
"""

import argparse
import os
import sys
import time
import mysql.connector
from mysql.connector import Error
import pandas as pd
from datetime import datetime, timedelta

# Historical data: yfinance (free, no API key). Uses NSE symbols e.g. INFY.NS.
try:
    import yfinance as yf
except ImportError:
    yf = None

# Nifty 50 stocks list (as of 2024)
NIFTY_50_STOCKS = [
    {"symbol": "RELIANCE", "name": "Reliance Industries Ltd", "sector": "Energy"},
    {"symbol": "TCS", "name": "Tata Consultancy Services Ltd", "sector": "Information Technology"},
    {"symbol": "HDFCBANK", "name": "HDFC Bank Ltd", "sector": "Financial Services"},
    {"symbol": "INFY", "name": "Infosys Ltd", "sector": "Information Technology"},
    {"symbol": "HINDUNILVR", "name": "Hindustan Unilever Ltd", "sector": "Consumer Goods"},
    {"symbol": "ICICIBANK", "name": "ICICI Bank Ltd", "sector": "Financial Services"},
    {"symbol": "BHARTIARTL", "name": "Bharti Airtel Ltd", "sector": "Telecommunications"},
    {"symbol": "SBIN", "name": "State Bank of India", "sector": "Financial Services"},
    {"symbol": "BAJFINANCE", "name": "Bajaj Finance Ltd", "sector": "Financial Services"},
    {"symbol": "LICI", "name": "Life Insurance Corporation of India", "sector": "Financial Services"},
    {"symbol": "ITC", "name": "ITC Ltd", "sector": "Consumer Goods"},
    {"symbol": "LT", "name": "Larsen & Toubro Ltd", "sector": "Construction"},
    {"symbol": "AXISBANK", "name": "Axis Bank Ltd", "sector": "Financial Services"},
    {"symbol": "KOTAKBANK", "name": "Kotak Mahindra Bank Ltd", "sector": "Financial Services"},
    {"symbol": "ASIANPAINT", "name": "Asian Paints Ltd", "sector": "Consumer Goods"},
    {"symbol": "TITAN", "name": "Titan Company Ltd", "sector": "Consumer Goods"},
    {"symbol": "ULTRACEMCO", "name": "UltraTech Cement Ltd", "sector": "Construction Materials"},
    {"symbol": "MARUTI", "name": "Maruti Suzuki India Ltd", "sector": "Automobile"},
    {"symbol": "SUNPHARMA", "name": "Sun Pharmaceutical Industries Ltd", "sector": "Pharmaceuticals"},
    {"symbol": "HCLTECH", "name": "HCL Technologies Ltd", "sector": "Information Technology"},
    {"symbol": "NTPC", "name": "NTPC Ltd", "sector": "Power"},
    {"symbol": "ONGC", "name": "Oil and Natural Gas Corporation Ltd", "sector": "Energy"},
    {"symbol": "POWERGRID", "name": "Power Grid Corporation of India Ltd", "sector": "Power"},
    {"symbol": "NESTLEIND", "name": "Nestle India Ltd", "sector": "Consumer Goods"},
    {"symbol": "M&M", "name": "Mahindra & Mahindra Ltd", "sector": "Automobile"},
    {"symbol": "WIPRO", "name": "Wipro Ltd", "sector": "Information Technology"},
    {"symbol": "COALINDIA", "name": "Coal India Ltd", "sector": "Energy"},
    {"symbol": "TATAMOTORS", "name": "Tata Motors Ltd", "sector": "Automobile"},
    {"symbol": "JSWSTEEL", "name": "JSW Steel Ltd", "sector": "Metals"},
    {"symbol": "ADANIENT", "name": "Adani Enterprises Ltd", "sector": "Diversified"},
    {"symbol": "TATASTEEL", "name": "Tata Steel Ltd", "sector": "Metals"},
    {"symbol": "HINDALCO", "name": "Hindalco Industries Ltd", "sector": "Metals"},
    {"symbol": "DIVISLAB", "name": "Dr. Reddy's Laboratories Ltd", "sector": "Pharmaceuticals"},
    {"symbol": "CIPLA", "name": "Cipla Ltd", "sector": "Pharmaceuticals"},
    {"symbol": "GRASIM", "name": "Grasim Industries Ltd", "sector": "Construction Materials"},
    {"symbol": "BAJAJFINSV", "name": "Bajaj Finserv Ltd", "sector": "Financial Services"},
    {"symbol": "ADANIPORTS", "name": "Adani Ports and Special Economic Zone Ltd", "sector": "Services"},
    {"symbol": "TECHM", "name": "Tech Mahindra Ltd", "sector": "Information Technology"},
    {"symbol": "HEROMOTOCO", "name": "Hero MotoCorp Ltd", "sector": "Automobile"},
    {"symbol": "BPCL", "name": "Bharat Petroleum Corporation Ltd", "sector": "Energy"},
    {"symbol": "IOC", "name": "Indian Oil Corporation Ltd", "sector": "Energy"},
    {"symbol": "EICHERMOT", "name": "Eicher Motors Ltd", "sector": "Automobile"},
    {"symbol": "BRITANNIA", "name": "Britannia Industries Ltd", "sector": "Consumer Goods"},
    {"symbol": "VEDL", "name": "Vedanta Ltd", "sector": "Metals"},
    {"symbol": "GODREJCP", "name": "Godrej Consumer Products Ltd", "sector": "Consumer Goods"},
    {"symbol": "DABUR", "name": "Dabur India Ltd", "sector": "Consumer Goods"},
    {"symbol": "HDFCLIFE", "name": "HDFC Life Insurance Company Ltd", "sector": "Financial Services"},
    {"symbol": "SBILIFE", "name": "SBI Life Insurance Company Ltd", "sector": "Financial Services"},
    {"symbol": "APOLLOHOSP", "name": "Apollo Hospitals Enterprise Ltd", "sector": "Healthcare"},
    {"symbol": "DLF", "name": "DLF Ltd", "sector": "Real Estate"},
]


def get_db_connection():
    """Create and return MySQL database connection."""
    try:
        connection = mysql.connector.connect(
            host=os.getenv('DB_HOST', 'localhost'),
            port=int(os.getenv('DB_PORT', '3306')),
            user=os.getenv('DB_USER', 'trader'),
            password=os.getenv('DB_PASSWORD', 'traderpass'),
            database=os.getenv('DB_NAME', 'algo_trader')
        )
        return connection
    except Error as e:
        print(f"Error connecting to MySQL: {e}")
        sys.exit(1)


def insert_stock(connection, stock_info):
    """Insert or update stock metadata."""
    cursor = connection.cursor()
    nse_symbol = f"{stock_info['symbol']}.NS"
    
    query = """
        INSERT INTO stocks (symbol, name, sector, nse_symbol)
        VALUES (%s, %s, %s, %s)
        ON DUPLICATE KEY UPDATE
            name = VALUES(name),
            sector = VALUES(sector),
            nse_symbol = VALUES(nse_symbol)
    """
    
    cursor.execute(query, (
        stock_info['symbol'],
        stock_info['name'],
        stock_info['sector'],
        nse_symbol
    ))
    connection.commit()
    stock_id = cursor.lastrowid
    
    # If it was an update, get the existing ID
    if stock_id == 0:
        cursor.execute("SELECT id FROM stocks WHERE symbol = %s", (stock_info['symbol'],))
        result = cursor.fetchone()
        stock_id = result[0] if result else None
    
    cursor.close()
    return stock_id


def insert_stock_prices(connection, stock_id, historical_data):
    """Insert historical stock prices (dma_20 computed and updated after insert)."""
    cursor = connection.cursor()
    
    query = """
        INSERT INTO stock_prices (stock_id, date, open, high, low, close, volume)
        VALUES (%s, %s, %s, %s, %s, %s, %s)
        ON DUPLICATE KEY UPDATE
            open = VALUES(open),
            high = VALUES(high),
            low = VALUES(low),
            close = VALUES(close),
            volume = VALUES(volume)
    """
    
    inserted_count = 0
    for date, row in historical_data.iterrows():
        try:
            # Handle different column name formats
            # NSEDownload uses: Open, High, Low, Close, Volume
            # yfinance uses: Open, High, Low, Close, Volume
            # But NSEDownload might have different capitalization
            open_price = float(row.get('Open', row.get('open', 0)))
            high_price = float(row.get('High', row.get('high', 0)))
            low_price = float(row.get('Low', row.get('low', 0)))
            close_price = float(row.get('Close', row.get('close', 0)))
            volume = int(row.get('Volume', row.get('volume', 0))) if not pd.isna(row.get('Volume', row.get('volume', 0))) else 0
            
            # Convert date index to date object
            if isinstance(date, pd.Timestamp):
                date_obj = date.date()
            else:
                date_obj = date if isinstance(date, datetime) else datetime.strptime(str(date), '%Y-%m-%d').date()
            
            cursor.execute(query, (
                stock_id,
                date_obj,
                open_price,
                high_price,
                low_price,
                close_price,
                volume
            ))
            inserted_count += 1
        except Exception as e:
            print(f"  Error inserting data for {date}: {e}")
            continue
    
    connection.commit()
    cursor.close()
    return inserted_count


def update_dma_20_for_stock(connection, stock_id):
    """Compute and save 20-day moving average for all rows of a stock that have 20+ days of history."""
    cursor = connection.cursor()
    cursor.execute(
        "SELECT id, date, close FROM stock_prices WHERE stock_id = %s ORDER BY date",
        (stock_id,)
    )
    rows = cursor.fetchall()
    if len(rows) < 20:
        cursor.close()
        return 0
    updated = 0
    for i in range(19, len(rows)):
        row_id, _, _ = rows[i]
        closes = [float(r[2]) for r in rows[i - 19 : i + 1]]
        dma_20 = sum(closes) / 20.0
        cursor.execute("UPDATE stock_prices SET dma_20 = %s WHERE id = %s", (round(dma_20, 2), row_id))
        updated += 1
    connection.commit()
    cursor.close()
    return updated


def fetch_and_load_stock(connection, stock_info):
    """Fetch historical data for a stock and load into database."""
    symbol = stock_info['symbol']
    nse_symbol = f"{symbol}.NS"
    
    print(f"\nProcessing {symbol} ({stock_info['name']})...")
    
    # Insert/update stock metadata
    stock_id = insert_stock(connection, stock_info)
    if not stock_id:
        print(f"  Failed to get stock_id for {symbol}")
        return False
    
    # Free source: yfinance (NSE symbols e.g. INFY.NS), no API key
    hist = None
    if yf is not None:
        try:
            print(f"  Fetching from Yahoo Finance ({nse_symbol})...")
            ticker = yf.Ticker(nse_symbol)
            hist = ticker.history(period="1y")
            if hist.empty:
                end_date = datetime.now()
                start_date = end_date - timedelta(days=365)
                hist = ticker.history(start=start_date, end=end_date)
            if hist.empty:
                hist = ticker.history(period="2y")
                if not hist.empty:
                    hist = hist[hist.index >= (datetime.now() - timedelta(days=365)).date()]
        except Exception as e:
            print(f"  Yahoo Finance failed: {e}")
            hist = None

    if hist is None or hist.empty:
        print(f"  No historical data found for {symbol}")
        return False

    try:
        # Insert prices
        count = insert_stock_prices(connection, stock_id, hist)
        print(f"  ✓ Inserted {count} records for {symbol}")
        
        # Compute and save 20 DMA for each day
        dma_updated = update_dma_20_for_stock(connection, stock_id)
        if dma_updated > 0:
            print(f"  ✓ Updated 20 DMA for {dma_updated} days")
        
        # Small delay to avoid rate limiting
        time.sleep(0.5)
        return True
        
    except Exception as e:
        print(f"  ✗ Error fetching data for {nse_symbol}: {e}")
        return False


def backfill_dma20_all(connection):
    """Update 20 DMA for all stocks that have price history (for existing DBs)."""
    cursor = connection.cursor()
    cursor.execute("SELECT id, symbol FROM stocks ORDER BY symbol")
    stocks = cursor.fetchall()
    cursor.close()
    total_updated = 0
    for stock_id, symbol in stocks:
        n = update_dma_20_for_stock(connection, stock_id)
        if n > 0:
            print(f"  {symbol}: updated {n} days")
            total_updated += n
    return total_updated


def main():
    parser = argparse.ArgumentParser(description="Load Nifty 50 historical data and/or backfill 20 DMA")
    parser.add_argument("--backfill-dma-only", action="store_true", help="Only compute and save 20 DMA for existing stock_prices")
    args = parser.parse_args()

    print("=" * 60)
    if args.backfill_dma_only:
        print("Backfill 20 DMA for all stocks")
        print("=" * 60)
        connection = get_db_connection()
        print("✓ Database connection established")
        total = backfill_dma20_all(connection)
        connection.close()
        print(f"\n✓ Backfill complete: {total} rows updated with 20 DMA")
        return
    print("NSE Nifty 50 Historical Data Loader")
    print("=" * 60)

    # Connect to database
    print("\nConnecting to database...")
    connection = get_db_connection()
    print("✓ Database connection established")

    # Process each stock
    success_count = 0
    fail_count = 0

    for stock_info in NIFTY_50_STOCKS:
        if fetch_and_load_stock(connection, stock_info):
            success_count += 1
        else:
            fail_count += 1

    # Close connection
    connection.close()

    print("\n" + "=" * 60)
    print(f"Data loading complete!")
    print(f"  Success: {success_count} stocks")
    print(f"  Failed: {fail_count} stocks")
    print("=" * 60)


if __name__ == "__main__":
    main()
