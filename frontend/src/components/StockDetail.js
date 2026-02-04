import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, AreaChart, Area } from 'recharts';
import { getStockHistory } from '../services/api';

const StockDetail = () => {
  const { symbol } = useParams();
  const navigate = useNavigate();
  const [history, setHistory] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [timeRange, setTimeRange] = useState('1y');

  useEffect(() => {
    fetchHistory();
  }, [symbol, timeRange]);

  const fetchHistory = async () => {
    try {
      setLoading(true);
      const endDate = new Date();
      const startDate = new Date();
      
      if (timeRange === '1y') {
        startDate.setFullYear(endDate.getFullYear() - 1);
      } else if (timeRange === '6m') {
        startDate.setMonth(endDate.getMonth() - 6);
      } else if (timeRange === '3m') {
        startDate.setMonth(endDate.getMonth() - 3);
      } else if (timeRange === '1m') {
        startDate.setMonth(endDate.getMonth() - 1);
      }

      const data = await getStockHistory(
        symbol,
        startDate.toISOString().split('T')[0],
        endDate.toISOString().split('T')[0]
      );
      
      // Transform data for chart
      const chartData = Array.isArray(data) ? data.map(item => ({
        date: new Date(item.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
        fullDate: item.date,
        open: parseFloat(item.open),
        high: parseFloat(item.high),
        low: parseFloat(item.low),
        close: parseFloat(item.close),
        volume: parseInt(item.volume) || 0,
      })).reverse() : [];
      
      setHistory(chartData);
      setError(null);
    } catch (err) {
      setError('Failed to fetch historical data.');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const formatCurrency = (value) => {
    return `₹${value.toFixed(2)}`;
  };

  const formatVolume = (value) => {
    if (value >= 10000000) {
      return `${(value / 10000000).toFixed(2)}Cr`;
    } else if (value >= 100000) {
      return `${(value / 100000).toFixed(2)}L`;
    }
    return value.toLocaleString();
  };

  if (loading) {
    return (
      <div className="min-h-screen relative z-10 flex justify-center items-center">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-[#00d4ff] mb-4"></div>
          <div className="text-xl text-[#e5e5e5]">Loading historical data...</div>
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
            <div className="flex items-center gap-4">
              <button
                onClick={() => navigate('/')}
                className="px-4 py-2 glass rounded-lg border border-[#2a2a2a] hover:border-[#00d4ff]/30 transition-all text-[#e5e5e5]"
              >
                ← Back
              </button>
              <div>
                <h1 className="text-3xl font-bold gradient-text mb-1">{symbol}</h1>
                <p className="text-sm text-[#888]">Historical Price Chart</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {['1m', '3m', '6m', '1y'].map((range) => (
                <button
                  key={range}
                  onClick={() => setTimeRange(range)}
                  className={`px-4 py-2 rounded-lg font-medium transition-all ${
                    timeRange === range
                      ? 'bg-gradient-to-r from-[#00d4ff] to-[#5b8def] text-white'
                      : 'glass border border-[#2a2a2a] text-[#e5e5e5] hover:border-[#00d4ff]/30'
                  }`}
                >
                  {range.toUpperCase()}
                </button>
              ))}
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

        {history.length === 0 ? (
          <div className="glass rounded-xl p-12 text-center border border-[#2a2a2a]">
            <div className="text-4xl mb-4">📊</div>
            <div className="text-xl text-[#e5e5e5] mb-2">No Historical Data</div>
            <div className="text-sm text-[#888]">
              Historical price data is not available for {symbol} yet.
            </div>
          </div>
        ) : (
          <>
            {/* Price Chart */}
            <div className="glass rounded-xl p-6 mb-6 border border-[#2a2a2a]">
              <h2 className="text-xl font-bold text-[#e5e5e5] mb-4">Price Chart</h2>
              <ResponsiveContainer width="100%" height={400}>
                <AreaChart data={history} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="colorClose" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#00d4ff" stopOpacity={0.3}/>
                      <stop offset="95%" stopColor="#00d4ff" stopOpacity={0}/>
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#2a2a2a" />
                  <XAxis 
                    dataKey="date" 
                    stroke="#888"
                    style={{ fontSize: '12px' }}
                    interval="preserveStartEnd"
                  />
                  <YAxis 
                    stroke="#888"
                    style={{ fontSize: '12px' }}
                    tickFormatter={(value) => `₹${value.toFixed(0)}`}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: '#1a1a1a',
                      border: '1px solid #2a2a2a',
                      borderRadius: '8px',
                      color: '#e5e5e5'
                    }}
                    formatter={(value, name) => {
                      if (name === 'close' || name === 'open' || name === 'high' || name === 'low') {
                        return formatCurrency(value);
                      }
                      return formatVolume(value);
                    }}
                    labelStyle={{ color: '#00d4ff', fontWeight: 'bold' }}
                  />
                  <Legend 
                    wrapperStyle={{ color: '#e5e5e5' }}
                    iconType="line"
                  />
                  <Area
                    type="monotone"
                    dataKey="close"
                    stroke="#00d4ff"
                    strokeWidth={2}
                    fillOpacity={1}
                    fill="url(#colorClose)"
                    name="Close Price"
                  />
                  <Line
                    type="monotone"
                    dataKey="high"
                    stroke="#00ff88"
                    strokeWidth={1}
                    strokeDasharray="5 5"
                    dot={false}
                    name="High"
                  />
                  <Line
                    type="monotone"
                    dataKey="low"
                    stroke="#ff4444"
                    strokeWidth={1}
                    strokeDasharray="5 5"
                    dot={false}
                    name="Low"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>

            {/* Volume Chart */}
            <div className="glass rounded-xl p-6 mb-6 border border-[#2a2a2a]">
              <h2 className="text-xl font-bold text-[#e5e5e5] mb-4">Volume</h2>
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={history} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="colorVolume" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#5b8def" stopOpacity={0.3}/>
                      <stop offset="95%" stopColor="#5b8def" stopOpacity={0}/>
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#2a2a2a" />
                  <XAxis 
                    dataKey="date" 
                    stroke="#888"
                    style={{ fontSize: '12px' }}
                    interval="preserveStartEnd"
                  />
                  <YAxis 
                    stroke="#888"
                    style={{ fontSize: '12px' }}
                    tickFormatter={(value) => formatVolume(value)}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: '#1a1a1a',
                      border: '1px solid #2a2a2a',
                      borderRadius: '8px',
                      color: '#e5e5e5'
                    }}
                    formatter={(value) => formatVolume(value)}
                  />
                  <Area
                    type="monotone"
                    dataKey="volume"
                    stroke="#5b8def"
                    strokeWidth={2}
                    fillOpacity={1}
                    fill="url(#colorVolume)"
                    name="Volume"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>

            {/* Statistics */}
            {history.length > 0 && (
              <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                <div className="glass rounded-xl p-4 border border-[#2a2a2a]">
                  <div className="text-sm text-[#888] mb-2">Current Price</div>
                  <div className="text-2xl font-bold text-[#e5e5e5]">
                    {formatCurrency(history[history.length - 1].close)}
                  </div>
                </div>
                <div className="glass rounded-xl p-4 border border-[#2a2a2a]">
                  <div className="text-sm text-[#888] mb-2">52 Week High</div>
                  <div className="text-2xl font-bold text-[#00ff88]">
                    {formatCurrency(Math.max(...history.map(h => h.high)))}
                  </div>
                </div>
                <div className="glass rounded-xl p-4 border border-[#2a2a2a]">
                  <div className="text-sm text-[#888] mb-2">52 Week Low</div>
                  <div className="text-2xl font-bold text-[#ff4444]">
                    {formatCurrency(Math.min(...history.map(h => h.low)))}
                  </div>
                </div>
                <div className="glass rounded-xl p-4 border border-[#2a2a2a]">
                  <div className="text-sm text-[#888] mb-2">Avg Volume</div>
                  <div className="text-2xl font-bold text-[#e5e5e5]">
                    {formatVolume(Math.round(history.reduce((sum, h) => sum + h.volume, 0) / history.length))}
                  </div>
                </div>
              </div>
            )}
          </>
        )}
      </main>
    </div>
  );
};

export default StockDetail;
