import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { getPositions, getPositionsSummary, getTrades, getStatus } from '../services/api';

const TokenBadge = ({ valid }) => (
  <div className={`flex items-center gap-2 glass rounded-lg px-3 py-2 border ${valid ? 'border-[#00ff88]/30' : 'border-[#ff4444]/50 bg-[#ff4444]/10'}`}>
    <div className={`w-2 h-2 rounded-full ${valid ? 'bg-[#00ff88]' : 'bg-[#ff4444] animate-pulse'}`}></div>
    <span className={`text-xs font-medium ${valid ? 'text-[#00ff88]' : 'text-[#ff4444]'}`}>
      {valid ? 'Kite Connected' : 'Kite Token Expired'}
    </span>
  </div>
);

const Portfolio = () => {
  const navigate = useNavigate();
  const [summary, setSummary] = useState(null);
  const [openPositions, setOpenPositions] = useState([]);
  const [tokenValid, setTokenValid] = useState(null);
  const [closedPositions, setClosedPositions] = useState([]);
  const [trades, setTrades] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showClosed, setShowClosed] = useState(false);

  const fetchData = async () => {
    try {
      setLoading(true);
      const [summaryData, openData, closedData, tradesData] = await Promise.all([
        getPositionsSummary(),
        getPositions('OPEN'),
        getPositions('CLOSED'),
        getTrades(20),
      ]);
      setSummary(summaryData);
      setOpenPositions(openData);
      setClosedPositions(closedData);
      setTrades(tradesData);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 5 * 60 * 1000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const checkToken = async () => {
      const status = await getStatus();
      setTokenValid(status.kite_token_valid);
    };
    checkToken();
    const interval = setInterval(checkToken, 5 * 60 * 1000);
    return () => clearInterval(interval);
  }, []);

  const fmt = (v, decimals = 2) => {
    if (v == null) return 'N/A';
    return `₹${v.toLocaleString('en-IN', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })}`;
  };

  const pnlColor = (v) => {
    if (v == null) return 'text-[#888]';
    return v >= 0 ? 'text-[#00ff88]' : 'text-[#ff4444]';
  };

  const pnlSign = (v) => (v >= 0 ? '+' : '');

  // Compute totals for holdings table
  const totals = openPositions.reduce(
    (acc, p) => ({
      buyAmount: acc.buyAmount + p.buy_amount,
      currentValue: acc.currentValue + (p.current_value || 0),
      unrealizedPnl: acc.unrealizedPnl + (p.unrealized_pnl || 0),
    }),
    { buyAmount: 0, currentValue: 0, unrealizedPnl: 0 }
  );

  if (loading && !summary) {
    return (
      <div className="flex justify-center items-center h-screen relative z-10">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-[#00d4ff] mb-4"></div>
          <div className="text-xl text-[#e5e5e5]">Loading portfolio...</div>
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
              <h1 className="text-3xl font-bold gradient-text mb-1">Portfolio</h1>
              <p className="text-sm text-[#888]">Virtual trading dashboard</p>
            </div>
            <div className="flex items-center gap-4">
              {tokenValid !== null && <TokenBadge valid={tokenValid} />}
              <button
                onClick={() => navigate('/')}
                className="px-5 py-2.5 glass rounded-lg border border-[#2a2a2a] text-[#e5e5e5] font-medium hover:border-[#00d4ff]/50 transition-all"
              >
                Stocks
              </button>
              <button
                onClick={fetchData}
                disabled={loading}
                className="px-6 py-2.5 bg-gradient-to-r from-[#00d4ff] to-[#5b8def] text-white rounded-lg font-medium hover:opacity-90 disabled:opacity-50 transition-all duration-200 shadow-lg shadow-[#00d4ff]/20"
              >
                {loading ? 'Refreshing...' : 'Refresh'}
              </button>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-6 py-8">
        {/* Summary Cards */}
        {summary && (
          <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
            <div className="glass rounded-xl p-6 border border-[#2a2a2a]">
              <div className="text-sm text-[#888] mb-2">Total Invested</div>
              <div className="text-2xl font-bold text-[#e5e5e5]">{fmt(summary.total_invested, 0)}</div>
              <div className="text-xs text-[#888] mt-1">{summary.open_count} open positions</div>
            </div>
            <div className="glass rounded-xl p-6 border border-[#2a2a2a]">
              <div className="text-sm text-[#888] mb-2">Current Value</div>
              <div className="text-2xl font-bold text-[#e5e5e5]">{fmt(summary.current_value, 0)}</div>
            </div>
            <div className="glass rounded-xl p-6 border border-[#2a2a2a]">
              <div className="text-sm text-[#888] mb-2">Unrealized P&L</div>
              <div className={`text-2xl font-bold ${pnlColor(summary.unrealized_pnl)}`}>
                {pnlSign(summary.unrealized_pnl)}{fmt(summary.unrealized_pnl, 0)}
              </div>
              <div className={`text-sm mt-1 ${pnlColor(summary.unrealized_pct)}`}>
                {pnlSign(summary.unrealized_pct)}{summary.unrealized_pct?.toFixed(1)}%
              </div>
            </div>
            <div className="glass rounded-xl p-6 border border-[#2a2a2a]">
              <div className="text-sm text-[#888] mb-2">Realized P&L</div>
              <div className={`text-2xl font-bold ${pnlColor(summary.realized_pnl)}`}>
                {pnlSign(summary.realized_pnl)}{fmt(summary.realized_pnl, 0)}
              </div>
              <div className="text-xs text-[#888] mt-1">{summary.closed_count} closed trades</div>
            </div>
          </div>
        )}

        {/* Holdings Table */}
        <div className="glass rounded-xl border border-[#2a2a2a] overflow-hidden mb-8">
          <div className="px-6 py-4 border-b border-[#2a2a2a]">
            <h2 className="text-lg font-semibold text-[#e5e5e5]">Holdings</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b border-[#2a2a2a]">
                  {['Symbol', 'Company', 'Qty', 'Buy Price', 'Buy Amount', 'Current Price', 'Current Value', 'P&L', 'P&L %', 'Buy Date'].map(h => (
                    <th key={h} className="px-5 py-3 text-right text-xs font-semibold text-[#888] uppercase tracking-wider first:text-left [&:nth-child(2)]:text-left">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-[#2a2a2a]">
                {openPositions.length === 0 ? (
                  <tr>
                    <td colSpan="10" className="px-6 py-12 text-center text-[#888]">
                      No open positions
                    </td>
                  </tr>
                ) : (
                  <>
                    {openPositions.map((p) => {
                      const upnl = p.unrealized_pnl || 0;
                      const upnlPct = p.buy_amount > 0 ? (upnl / p.buy_amount) * 100 : 0;
                      const highlight = upnl > 0
                        ? 'bg-[#00ff88]/5'
                        : upnl < 0 ? 'bg-[#ff4444]/5' : '';
                      return (
                        <tr key={p.id} className={`${highlight} hover:bg-[#1a1a1a]/50 transition-colors`}>
                          <td className="px-5 py-3 text-sm font-bold text-[#e5e5e5]">{p.symbol}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5]">{p.name}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{p.quantity}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(p.buy_price)}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(p.buy_amount)}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(p.current_price)}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(p.current_value)}</td>
                          <td className={`px-5 py-3 text-sm text-right font-mono font-semibold ${pnlColor(upnl)}`}>
                            {pnlSign(upnl)}{fmt(upnl)}
                          </td>
                          <td className={`px-5 py-3 text-sm text-right font-mono font-semibold ${pnlColor(upnl)}`}>
                            {pnlSign(upnlPct)}{upnlPct.toFixed(1)}%
                          </td>
                          <td className="px-5 py-3 text-sm text-[#888] text-right">{p.buy_date}</td>
                        </tr>
                      );
                    })}
                    {/* Totals row */}
                    <tr className="border-t-2 border-[#444] bg-[#1a1a1a]/30">
                      <td className="px-5 py-3 text-sm font-bold text-[#e5e5e5]" colSpan="4">Total</td>
                      <td className="px-5 py-3 text-sm font-bold text-[#e5e5e5] text-right font-mono">{fmt(totals.buyAmount)}</td>
                      <td className="px-5 py-3"></td>
                      <td className="px-5 py-3 text-sm font-bold text-[#e5e5e5] text-right font-mono">{fmt(totals.currentValue)}</td>
                      <td className={`px-5 py-3 text-sm font-bold text-right font-mono ${pnlColor(totals.unrealizedPnl)}`}>
                        {pnlSign(totals.unrealizedPnl)}{fmt(totals.unrealizedPnl)}
                      </td>
                      <td className={`px-5 py-3 text-sm font-bold text-right font-mono ${pnlColor(totals.unrealizedPnl)}`}>
                        {totals.buyAmount > 0 ? `${pnlSign(totals.unrealizedPnl / totals.buyAmount * 100)}${(totals.unrealizedPnl / totals.buyAmount * 100).toFixed(1)}%` : ''}
                      </td>
                      <td className="px-5 py-3"></td>
                    </tr>
                  </>
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Closed Trades */}
        <div className="glass rounded-xl border border-[#2a2a2a] overflow-hidden mb-8">
          <button
            onClick={() => setShowClosed(!showClosed)}
            className="w-full px-6 py-4 flex justify-between items-center hover:bg-[#1a1a1a]/30 transition-colors"
          >
            <h2 className="text-lg font-semibold text-[#e5e5e5]">
              Closed Trades ({closedPositions.length})
            </h2>
            <span className="text-[#888]">{showClosed ? '▲' : '▼'}</span>
          </button>
          {showClosed && (
            <div className="overflow-x-auto border-t border-[#2a2a2a]">
              <table className="min-w-full">
                <thead>
                  <tr className="border-b border-[#2a2a2a]">
                    {['Symbol', 'Qty', 'Buy Price', 'Sell Price', 'Buy Amount', 'Sell Amount', 'P&L', 'Buy Date', 'Sell Date', 'Days'].map(h => (
                      <th key={h} className="px-5 py-3 text-right text-xs font-semibold text-[#888] uppercase tracking-wider first:text-left">
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#2a2a2a]">
                  {closedPositions.length === 0 ? (
                    <tr>
                      <td colSpan="10" className="px-6 py-8 text-center text-[#888]">No closed trades yet</td>
                    </tr>
                  ) : (
                    closedPositions.map((p) => {
                      const days = p.buy_date && p.sell_date
                        ? Math.round((new Date(p.sell_date) - new Date(p.buy_date)) / 86400000)
                        : '';
                      return (
                        <tr key={p.id} className="hover:bg-[#1a1a1a]/50 transition-colors">
                          <td className="px-5 py-3 text-sm font-bold text-[#e5e5e5]">{p.symbol}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{p.quantity}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(p.buy_price)}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(p.sell_price)}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(p.buy_amount)}</td>
                          <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(p.sell_amount)}</td>
                          <td className={`px-5 py-3 text-sm text-right font-mono font-semibold ${pnlColor(p.pnl)}`}>
                            {p.pnl != null ? `${pnlSign(p.pnl)}${fmt(p.pnl)}` : 'N/A'}
                          </td>
                          <td className="px-5 py-3 text-sm text-[#888] text-right">{p.buy_date}</td>
                          <td className="px-5 py-3 text-sm text-[#888] text-right">{p.sell_date}</td>
                          <td className="px-5 py-3 text-sm text-[#888] text-right">{days}</td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Recent Activity */}
        <div className="glass rounded-xl border border-[#2a2a2a] overflow-hidden">
          <div className="px-6 py-4 border-b border-[#2a2a2a]">
            <h2 className="text-lg font-semibold text-[#e5e5e5]">Recent Activity</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b border-[#2a2a2a]">
                  {['Date', 'Type', 'Symbol', 'Company', 'Qty', 'Price', 'Amount'].map(h => (
                    <th key={h} className="px-5 py-3 text-right text-xs font-semibold text-[#888] uppercase tracking-wider first:text-left [&:nth-child(2)]:text-left [&:nth-child(3)]:text-left [&:nth-child(4)]:text-left">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-[#2a2a2a]">
                {trades.length === 0 ? (
                  <tr>
                    <td colSpan="7" className="px-6 py-8 text-center text-[#888]">No trades yet</td>
                  </tr>
                ) : (
                  trades.map((t) => (
                    <tr key={t.id} className="hover:bg-[#1a1a1a]/50 transition-colors">
                      <td className="px-5 py-3 text-sm text-[#888]">{t.trade_date}</td>
                      <td className="px-5 py-3">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-bold ${
                          t.trade_type === 'BUY'
                            ? 'bg-[#00ff88]/15 text-[#00ff88]'
                            : 'bg-[#ff4444]/15 text-[#ff4444]'
                        }`}>
                          {t.trade_type}
                        </span>
                      </td>
                      <td className="px-5 py-3 text-sm font-bold text-[#e5e5e5]">{t.symbol}</td>
                      <td className="px-5 py-3 text-sm text-[#e5e5e5]">{t.name}</td>
                      <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{t.quantity}</td>
                      <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(t.price)}</td>
                      <td className="px-5 py-3 text-sm text-[#e5e5e5] text-right font-mono">{fmt(t.amount)}</td>
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

export default Portfolio;
