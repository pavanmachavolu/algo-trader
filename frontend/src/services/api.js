import axios from 'axios';

// Use relative path so nginx can proxy to backend
const API_URL = process.env.REACT_APP_API_URL || '';

const api = axios.create({
  baseURL: API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

export const getStocks = async () => {
  try {
    const response = await api.get('/api/stocks');
    // Return the full response object (may contain metadata)
    return response.data;
  } catch (error) {
    console.error('Error fetching stocks:', error);
    throw error;
  }
};

export const getStockHistory = async (symbol, startDate = null, endDate = null) => {
  try {
    const params = {};
    if (startDate) params.start_date = startDate;
    if (endDate) params.end_date = endDate;
    
    const response = await api.get(`/api/stocks/${symbol}/history`, { params });
    return response.data;
  } catch (error) {
    console.error('Error fetching stock history:', error);
    throw error;
  }
};

export const getPositions = async (status = null) => {
  try {
    const params = {};
    if (status) params.status = status;
    const response = await api.get('/api/positions', { params });
    return response.data || [];
  } catch (error) {
    console.error('Error fetching positions:', error);
    throw error;
  }
};

export const getPositionsSummary = async () => {
  try {
    const response = await api.get('/api/positions/summary');
    return response.data;
  } catch (error) {
    console.error('Error fetching positions summary:', error);
    throw error;
  }
};

export const getTrades = async (limit = 20) => {
  try {
    const response = await api.get('/api/trades', { params: { limit } });
    return response.data || [];
  } catch (error) {
    console.error('Error fetching trades:', error);
    throw error;
  }
};

export const refreshPrices = async () => {
  const response = await api.post('/api/prices/refresh');
  return response.data;
};

export const getStatus = async () => {
  try {
    const response = await api.get('/api/status');
    return response.data;
  } catch (error) {
    console.error('Error fetching status:', error);
    return { kite_token_valid: false };
  }
};
