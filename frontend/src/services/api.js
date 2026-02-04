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
