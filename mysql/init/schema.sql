-- Create database if not exists
CREATE DATABASE IF NOT EXISTS algo_trader;
USE algo_trader;

-- Table for Nifty 50 stock metadata
CREATE TABLE IF NOT EXISTS stocks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    symbol VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(200) NOT NULL,
    sector VARCHAR(100),
    nse_symbol VARCHAR(50) NOT NULL COMMENT 'NSE symbol format (e.g., RELIANCE.NS)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_symbol (symbol),
    INDEX idx_nse_symbol (nse_symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Table for historical stock prices (OHLCV data)
CREATE TABLE IF NOT EXISTS stock_prices (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    stock_id INT NOT NULL,
    date DATE NOT NULL,
    open DECIMAL(15, 2) NOT NULL,
    high DECIMAL(15, 2) NOT NULL,
    low DECIMAL(15, 2) NOT NULL,
    close DECIMAL(15, 2) NOT NULL,
    volume BIGINT NOT NULL,
    dma_20 DECIMAL(15, 2) NULL COMMENT '20-day simple moving average of close',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_stock_date (stock_id, date),
    FOREIGN KEY (stock_id) REFERENCES stocks(id) ON DELETE CASCADE,
    INDEX idx_date (date),
    INDEX idx_stock_date (stock_id, date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Table for latest/delayed prices (updated every 5 minutes)
CREATE TABLE IF NOT EXISTS latest_prices (
    id INT AUTO_INCREMENT PRIMARY KEY,
    stock_id INT NOT NULL,
    price DECIMAL(15, 2) NOT NULL,
    change_percent DECIMAL(10, 4) NOT NULL,
    volume BIGINT NOT NULL,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY unique_stock (stock_id),
    FOREIGN KEY (stock_id) REFERENCES stocks(id) ON DELETE CASCADE,
    INDEX idx_last_updated (last_updated)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Virtual trader: open/closed positions
CREATE TABLE IF NOT EXISTS positions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    stock_id INT NOT NULL,
    quantity INT NOT NULL,
    buy_price DECIMAL(15, 2) NOT NULL,
    buy_amount DECIMAL(15, 2) NOT NULL,
    buy_date DATE NOT NULL,
    status ENUM('OPEN', 'CLOSED') NOT NULL DEFAULT 'OPEN',
    sell_price DECIMAL(15, 2) NULL,
    sell_amount DECIMAL(15, 2) NULL,
    sell_date DATE NULL,
    pnl DECIMAL(15, 2) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (stock_id) REFERENCES stocks(id) ON DELETE CASCADE,
    INDEX idx_status (status),
    INDEX idx_stock_status (stock_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Virtual trader: immutable trade log
CREATE TABLE IF NOT EXISTS trades (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    stock_id INT NOT NULL,
    trade_type ENUM('BUY', 'SELL') NOT NULL,
    quantity INT NOT NULL,
    price DECIMAL(15, 2) NOT NULL,
    amount DECIMAL(15, 2) NOT NULL,
    trade_date DATE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (stock_id) REFERENCES stocks(id) ON DELETE CASCADE,
    INDEX idx_trade_date (trade_date),
    INDEX idx_stock_trade (stock_id, trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Virtual trader: one row per successful daily run (prevents duplicates)
CREATE TABLE IF NOT EXISTS trader_runs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    run_date DATE NOT NULL,
    buys INT NOT NULL DEFAULT 0,
    sells INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_run_date (run_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
