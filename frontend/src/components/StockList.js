import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { getStocks } from '../services/api';

const StockList = () => {
  const navigate = useNavigate();
  const [stocks, setStocks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [lastUpdated, setLastUpdated] = useState(null);

  const fetchStocks = async () => {
    try {
      setLoading(true);
      const response = await getStocks();
      
      // Handle both old format (array) and new format (object with metadata)
      if (Array.isArray(response)) {
        setStocks(response);
        setLastUpdated(new Date());
      } else {
        setStocks(response.stocks || response);
        // Use the backend's last_price_fetch timestamp
        if (response.metadata && response.metadata.last_price_fetch) {
          setLastUpdated(new Date(response.metadata.last_price_fetch));
        } else {
          setLastUpdated(new Date());
        }
      }
      setError(null);
    } catch (err) {
      setError('Failed to fetch stocks. Please try again later.');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStocks();
    
    // Auto-refresh every 5 minutes
    const interval = setInterval(fetchStocks, 5 * 60 * 1000);
    
    return () => clearInterval(interval);
  }, []);

  const formatPrice = (price) => {
    if (!price) return 'N/A';
    return `₹${price.toFixed(2)}`;
  };

  const formatChange = (changePercent) => {
    if (changePercent === null || changePercent === undefined) return 'N/A';
    const sign = changePercent >= 0 ? '+' : '';
    const isPositive = changePercent >= 0;
    return (
      <span className={`font-semibold ${isPositive ? 'text-[#00ff88]' : 'text-[#ff4444]'}`}>
        {sign}{changePercent.toFixed(2)}%
      </span>
    );
  };

  const formatVolume = (volume) => {
    if (!volume) return 'N/A';
    if (volume >= 10000000) {
      return `${(volume / 10000000).toFixed(2)}Cr`;
    } else if (volume >= 100000) {
      return `${(volume / 100000).toFixed(2)}L`;
    }
    return volume.toLocaleString();
  };

  if (loading && stocks.length === 0) {
    return (
      <div className="flex justify-center items-center h-screen relative z-10">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-[#00d4ff] mb-4"></div>
          <div className="text-xl text-[#e5e5e5]">Loading stocks...</div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen relative z-10">
      {/* Header */}
      <header className="glass border-b border-[#2a2a2a] sticky top-0 z-20">
        <div className="container mx-auto px-6 py-6">
          <div className="flex justify-between items-center">
            <div>
              <h1 className="text-3xl font-bold gradient-text mb-1">Nifty 50</h1>
              <p className="text-sm text-[#888]">Real-time market data & analytics</p>
            </div>
            <div className="flex items-center gap-6">
              {lastUpdated && (
                <div className="text-right glass rounded-lg px-4 py-2 border border-[#2a2a2a]">
                  <div className="text-xs text-[#888] mb-1 uppercase tracking-wider">Last Price Fetch</div>
                  <div className="text-sm text-[#e5e5e5] font-mono font-semibold">
                    {lastUpdated.toLocaleString('en-US', { 
                      month: 'short', 
                      day: 'numeric', 
                      hour: '2-digit', 
                      minute: '2-digit',
                      second: '2-digit',
                      hour12: true
                    })}
                  </div>
                  <div className="text-xs text-[#888] mt-1">
                    {Math.round((new Date() - lastUpdated) / 1000 / 60)} min ago
                  </div>
                </div>
              )}
              <button
                onClick={fetchStocks}
                disabled={loading}
                className="px-6 py-2.5 bg-gradient-to-r from-[#00d4ff] to-[#5b8def] text-white rounded-lg font-medium hover:opacity-90 disabled:opacity-50 transition-all duration-200 shadow-lg shadow-[#00d4ff]/20"
              >
                {loading ? (
                  <span className="flex items-center gap-2">
                    <span className="inline-block animate-spin rounded-full h-4 w-4 border-t-2 border-b-2 border-white"></span>
                    Refreshing...
                  </span>
                ) : (
                  'Refresh'
                )}
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-6 py-8">
        {error && (
          <div className="mb-6 glass rounded-xl p-4 border border-[#ff4444]/30 bg-[#ff4444]/10">
            <div className="flex items-center gap-3">
              <div className="w-2 h-2 rounded-full bg-[#ff4444] animate-pulse"></div>
              <span className="text-[#ff4444]">{error}</span>
            </div>
          </div>
        )}

        {/* Stats Cards */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
          <div className="glass rounded-xl p-6 border border-[#2a2a2a] hover:border-[#00d4ff]/30 transition-all">
            <div className="text-sm text-[#888] mb-2">Total Stocks</div>
            <div className="text-3xl font-bold text-[#e5e5e5]">{stocks.length}</div>
          </div>
          <div className="glass rounded-xl p-6 border border-[#2a2a2a] hover:border-[#00d4ff]/30 transition-all">
            <div className="text-sm text-[#888] mb-2">Active Updates</div>
            <div className="text-3xl font-bold text-[#00ff88]">
              {stocks.filter(s => s.latest_price).length}
            </div>
          </div>
          <div className="glass rounded-xl p-6 border border-[#2a2a2a] hover:border-[#00d4ff]/30 transition-all">
            <div className="text-sm text-[#888] mb-2">Market Status</div>
            <div className="text-3xl font-bold text-[#00d4ff]">Live</div>
          </div>
        </div>

        {/* Stock Table */}
        <div className="glass rounded-xl border border-[#2a2a2a] overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b border-[#2a2a2a]">
                  <th className="px-6 py-4 text-left text-xs font-semibold text-[#888] uppercase tracking-wider">
                    Symbol
                  </th>
                  <th className="px-6 py-4 text-left text-xs font-semibold text-[#888] uppercase tracking-wider">
                    Company Name
                  </th>
                  <th className="px-6 py-4 text-left text-xs font-semibold text-[#888] uppercase tracking-wider">
                    Sector
                  </th>
                  <th className="px-6 py-4 text-right text-xs font-semibold text-[#888] uppercase tracking-wider">
                    Price
                  </th>
                  <th className="px-6 py-4 text-right text-xs font-semibold text-[#888] uppercase tracking-wider">
                    Change %
                  </th>
                  <th className="px-6 py-4 text-right text-xs font-semibold text-[#888] uppercase tracking-wider">
                    20 DMA
                  </th>
                  <th className="px-6 py-4 text-right text-xs font-semibold text-[#888] uppercase tracking-wider">
                    % vs 20 DMA
                  </th>
                  <th className="px-6 py-4 text-right text-xs font-semibold text-[#888] uppercase tracking-wider">
                    Volume
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#2a2a2a]">
                {stocks.length === 0 ? (
                  <tr>
                    <td colSpan="8" className="px-6 py-12 text-center text-[#888]">
                      <div className="flex flex-col items-center gap-3">
                        <div className="text-4xl">📊</div>
                        <div>No stocks available</div>
                      </div>
                    </td>
                  </tr>
                ) : (
                  stocks.map((stock, index) => (
                    <tr 
                      key={stock.id} 
                      className="hover:bg-[#1a1a1a]/50 transition-colors duration-150 cursor-pointer"
                      style={{ animationDelay: `${index * 20}ms` }}
                      onClick={() => navigate(`/stock/${stock.symbol}`)}
                    >
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm font-bold text-[#e5e5e5] hover:text-[#00d4ff] transition-colors">
                          {stock.symbol}
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="text-sm text-[#e5e5e5]">{stock.name}</div>
                      </td>
                      <td className="px-6 py-4">
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-[#2a2a2a] text-[#888]">
                          {stock.sector || 'N/A'}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right">
                        <div className="text-sm font-semibold text-[#e5e5e5]">
                          {formatPrice(stock.latest_price?.price)}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right">
                        {formatChange(stock.latest_price?.change_percent)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right">
                        <div className="text-sm text-[#e5e5e5] font-mono">
                          {formatPrice(stock.dma_20)}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right">
                        {stock.price_vs_dma20_percent != null ? formatChange(stock.price_vs_dma20_percent) : 'N/A'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right">
                        <div className="text-sm text-[#888] font-mono">
                          {formatVolume(stock.latest_price?.volume)}
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </main>
    </div>
  );
};

export default StockList;
