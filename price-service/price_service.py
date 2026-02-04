#!/usr/bin/env python3
"""
Price service for fetching NSE stock prices using nsetools.
This service provides HTTP API for the Go backend to fetch current prices.
"""

from flask import Flask, jsonify, request
from nsetools import Nse
import os

app = Flask(__name__)
nse = Nse()

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "ok"}), 200

@app.route('/quote', methods=['GET'])
def get_quote():
    """Get current quote for a stock symbol (without .NS suffix)"""
    symbol = request.args.get('symbol')
    if not symbol:
        return jsonify({"error": "symbol parameter required"}), 400
    
    # Remove .NS suffix if present
    symbol = symbol.replace('.NS', '').upper()
    
    try:
        quote = nse.get_quote(symbol)
        
        if not quote or 'lastPrice' not in quote:
            return jsonify({"error": f"No data found for {symbol}"}), 404
        
        # Extract relevant data
        last_price = float(quote.get('lastPrice', 0))
        previous_close = float(quote.get('previousClose', last_price))
        change = last_price - previous_close
        change_percent = (change / previous_close * 100) if previous_close > 0 else 0
        volume = int(quote.get('totalTradedVolume', 0))
        
        return jsonify({
            "symbol": symbol,
            "price": last_price,
            "change": change,
            "change_percent": change_percent,
            "volume": volume,
            "previous_close": previous_close,
            "open": float(quote.get('open', 0)),
            "high": float(quote.get('dayHigh', 0)),
            "low": float(quote.get('dayLow', 0))
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/quotes', methods=['POST'])
def get_quotes_batch():
    """Get quotes for multiple symbols at once"""
    data = request.get_json()
    symbols = data.get('symbols', [])
    
    if not symbols:
        return jsonify({"error": "symbols array required"}), 400
    
    results = []
    for symbol in symbols:
        symbol_clean = symbol.replace('.NS', '').upper()
        try:
            quote = nse.get_quote(symbol_clean)
            if quote and 'lastPrice' in quote:
                last_price = float(quote.get('lastPrice', 0))
                previous_close = float(quote.get('previousClose', last_price))
                change = last_price - previous_close
                change_percent = (change / previous_close * 100) if previous_close > 0 else 0
                volume = int(quote.get('totalTradedVolume', 0))
                
                results.append({
                    "symbol": symbol_clean,
                    "price": last_price,
                    "change_percent": change_percent,
                    "volume": volume
                })
        except Exception as e:
            # Continue with other symbols even if one fails
            continue
    
    return jsonify({"quotes": results}), 200

if __name__ == '__main__':
    port = int(os.getenv('PORT', '5000'))
    app.run(host='0.0.0.0', port=port, debug=False)
