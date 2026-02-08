#!/usr/bin/env python3
"""
Price service for Nifty 50 stock prices. Free, no API key: nsetools and direct NSE API.
"""

import os
import requests
from flask import Flask, jsonify, request

try:
    from nsetools import Nse
    nse = Nse()
except ImportError:
    nse = None

app = Flask(__name__)


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
    """Get current quote for a stock symbol (without .NS suffix). Uses nsetools and direct NSE API (free, no key)."""
    symbol = request.args.get('symbol')
    if not symbol:
        return jsonify({"error": "symbol parameter required"}), 400

    symbol_clean = symbol.replace('.NS', '').upper()

    try:
        quote = None
        if nse:
            try:
                quote = nse.get_quote(symbol_clean, all_data=False)
            except (KeyError, TypeError, Exception):
                quote = get_nse_quote_direct(symbol_clean)
                if not quote:
                    try:
                        quote = nse.get_quote(symbol_clean, all_data=True)
                    except Exception:
                        pass
        else:
            quote = get_nse_quote_direct(symbol_clean)

        if not quote:
            return jsonify({
                "error": f"No data found for {symbol_clean}",
                "note": "NSE symbol may be incorrect or API temporarily unavailable"
            }), 404

        # Normalize quote from NSE (lastPrice, open, high, low, totalTradedVolume)
        last_price = None
        for price_key in ['lastPrice', 'ltp', 'basePrice', 'price', 'closePrice']:
            if price_key in quote and quote[price_key] is not None:
                try:
                    last_price = float(quote[price_key])
                    break
                except (ValueError, TypeError):
                    continue
        if last_price is None or last_price == 0:
            return jsonify({"error": f"No valid price data found for {symbol_clean}"}), 404

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
        volume = 0
        for vol_key in ['totalTradedVolume', 'volume', 'tradedVolume', 'totalVolume']:
            if vol_key in quote and quote[vol_key] is not None:
                try:
                    volume = int(quote[vol_key])
                    break
                except (ValueError, TypeError):
                    continue

        return jsonify({
            "symbol": symbol_clean,
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
    """Get quotes for multiple symbols via nsetools / direct NSE (free, no API key)."""
    data = request.get_json() or {}
    symbols = data.get('symbols', [])

    if not symbols:
        return jsonify({"error": "symbols array required"}), 400

    results = []
    for symbol in symbols:
        symbol_clean = symbol.replace('.NS', '').upper()
        if nse:
            try:
                quote = nse.get_quote(symbol_clean)
                if quote and quote.get('lastPrice'):
                    last_price = float(quote.get('lastPrice', 0))
                    previous_close = float(quote.get('previousClose', last_price))
                    change_percent = ((last_price - previous_close) / previous_close * 100) if previous_close > 0 else 0
                    results.append({
                        "symbol": symbol_clean,
                        "price": last_price,
                        "change_percent": change_percent,
                        "volume": int(quote.get('totalTradedVolume', 0)),
                    })
            except Exception:
                pass
    return jsonify({"quotes": results}), 200

if __name__ == '__main__':
    port = int(os.getenv('PORT', '5000'))
    app.run(host='0.0.0.0', port=port, debug=False)
