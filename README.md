# NSE Nifty 50 Algorithmic Trading App

A full-stack application for monitoring and backtesting algorithmic trading strategies on NSE Nifty 50 stocks.

## Architecture

- **Backend**: Go REST API with MySQL database
- **Frontend**: React.js with Tailwind CSS
- **Database**: MySQL 8.0 with persistent Docker volumes
- **Data Source**: Free, no API key — Yahoo Finance (historical), nsetools / NSE (live quotes)
- **Deployment**: Docker Compose

## Features

- View all Nifty 50 stocks with latest prices
- Historical data storage (1 year) for backtesting
- Auto-refresh prices every 5 minutes
- Persistent data storage in Docker volumes
- Clean, responsive UI

## Prerequisites

- Docker and Docker Compose installed
- At least 2GB free disk space for database volume

## Quick Start

1. **Clone and navigate to the project:**
   ```bash
   cd algo-trader
   ```

2. **Start all services:**
   ```bash
   docker-compose up -d
   ```

3. **Load historical data (run once):**
   ```bash
   docker-compose --profile loader up data-loader
   ```

4. **Access the application:**
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8080
   - MySQL: localhost:3306

## Services

### MySQL Database
- Port: 3306
- Database: `algo_trader`
- User: `trader` / Password: `traderpass`
- Root Password: `rootpassword`
- Data persisted in `./docker-volumes/mysql-data`

### Go Backend
- Port: 8080
- Endpoints:
  - `GET /api/stocks` - List all stocks with latest prices
  - `GET /api/stocks/{symbol}/history` - Get historical data for a stock
  - `GET /health` - Health check
- **Price fetcher:** Fetches latest prices from Yahoo Finance (then NSE fallback) on startup and on a schedule. To avoid Yahoo 429 rate limits, a delay is applied between each symbol. Tune in code: `YahooFetchDelaySec` and `PriceFetchIntervalMin` in `backend/services/price_fetcher.go`.

### React Frontend
- Port: 3000
- Auto-refreshes every 5 minutes

### Python Data Loader
- One-time service to load 1 year of historical data (and 20 DMA) via Yahoo Finance (NSE symbols e.g. INFY.NS). No API key required.
- Run with: `docker-compose --profile loader up data-loader`

## API Usage

### Get all stocks
```bash
curl http://localhost:8080/api/stocks
```

### Get stock history
```bash
curl http://localhost:8080/api/stocks/RELIANCE/history
```

### Get stock history with date range
```bash
curl "http://localhost:8080/api/stocks/RELIANCE/history?start_date=2024-01-01&end_date=2024-12-31"
```

## Database Schema

- `stocks`: Stock metadata (symbol, name, sector)
- `stock_prices`: Historical OHLCV data
- `latest_prices`: Current/delayed prices (updated every 5 minutes)

## Development

### Backend Development
```bash
cd backend
go mod download
go run main.go
```

### Frontend Development
```bash
cd frontend
npm install
npm start
```

## Stopping Services

```bash
docker-compose down
```

To remove volumes (deletes all data):
```bash
docker-compose down -v
```

## Notes

- Price data is delayed (typically 15-20 minutes) as per Yahoo Finance API
- Historical data loading may take 10-15 minutes for all 50 stocks
- Database data persists in `./docker-volumes/mysql-data` even after container stops

## Troubleshooting

1. **Port already in use**: Change ports in `docker-compose.yml`
2. **Database connection errors**: Wait for MySQL to be healthy before starting other services
3. **No price data**: Ensure data-loader has completed successfully
4. **Frontend can't connect to backend**: Check `REACT_APP_API_URL` in frontend environment
