#!/usr/bin/env python3
"""
Price service for fetching NSE stock prices using nsetools.
This service provides HTTP API for the Go backend to fetch current prices.
"""

from flask import Flask, jsonify, request
from nsetools import Nse
import os
import requests
import json

app = Flask(__name__)
nse = Nse()

def get_nse_quote_direct(symbol):
    """Fallback method: Direct NSE API call for stocks nsetools can't handle"""
    try:
        # NSE's direct API endpoint
        url = f"https://www.nseindia.com/api/quote-equity?symbol={symbol}"
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
            'Accept': 'application/json',
            'Accept-Language': 'en-US,en;q=0.9'
        }
        response = requests.get(url, headers=headers, timeout=10)
        if response.status_code == 200:
            data = response.json()
            price_info = data.get('priceInfo', {})
            if price_info:
                return {
                    'lastPrice': price_info.get('lastPrice', 0),
                    'previousClose': price_info.get('previousClose', 0),
                    'open': price_info.get('open', 0),
                    'dayHigh': price_info.get('intraDayHighLow', {}).get('max', 0),
                    'dayLow': price_info.get('intraDayHighLow', {}).get('min', 0),
                    'totalTradedVolume': price_info.get('totalTradedVolume', 0)
                }
    except Exception as e:
        pass
    return None

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
        quote = None
        # Try nsetools first
        try:
            quote = nse.get_quote(symbol, all_data=False)
        except (KeyError, TypeError, Exception) as e:
            # If nsetools fails (common with TATAMOTORS, M&M), try direct NSE API
            quote = get_nse_quote_direct(symbol)
            if not quote:
                # Try with all_data=True as last resort
                try:
                    quote = nse.get_quote(symbol, all_data=True)
                except:
                    pass
        
        if not quote:
            return jsonify({
                "error": f"No data found for {symbol}",
                "note": "This stock may not be available via NSE API or symbol may be incorrect"
            }), 404
        
        # Try multiple possible price field names
        last_price = None
        for price_key in ['lastPrice', 'ltp', 'basePrice', 'price', 'closePrice']:
            if price_key in quote and quote[price_key] is not None:
                try:
                    last_price = float(quote[price_key])
                    break
                except (ValueError, TypeError):
                    continue
        
        if last_price is None or last_price == 0:
            return jsonify({"error": f"No valid price data found for {symbol}"}), 404
        
        # Try multiple possible previous close field names
        previous_close = last_price
        for close_key in ['previousClose', 'prevClose', 'closePrice', 'basePrice']:
            if close_key in quote and quote[close_key] is not None:
                try:
                    previous_close = float(quote[close_key])
                    if previous_close > 0:
                        break
                except (ValueError, TypeError):
                    continue
        
        change = last_price - previous_close
        change_percent = (change / previous_close * 100) if previous_close > 0 else 0
        
        # Try multiple possible volume field names
        volume = 0
        for vol_key in ['totalTradedVolume', 'volume', 'tradedVolume', 'totalVolume']:
            if vol_key in quote and quote[vol_key] is not None:
                try:
                    volume = int(quote[vol_key])
                    break
                except (ValueError, TypeError):
                    continue
        
        return jsonify({
            "symbol": symbol,
            "price": last_price,
            "change": change,
            "change_percent": change_percent,
            "volume": volume,
            "previous_close": previous_close,
            "open": float(quote.get('open', quote.get('dayOpen', 0)) or 0),
            "high": float(quote.get('dayHigh', quote.get('high', 0)) or 0),
            "low": float(quote.get('dayLow', quote.get('low', 0)) or 0)
        }), 200
        
    except Exception as e:
        import traceback
        error_details = traceback.format_exc()
        return jsonify({"error": str(e), "details": error_details}), 500

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
