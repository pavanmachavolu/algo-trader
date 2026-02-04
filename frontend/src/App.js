import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import StockList from './components/StockList';
import StockDetail from './components/StockDetail';
import './App.css';

function App() {
  return (
    <Router>
      <div className="App">
        <Routes>
          <Route path="/" element={<StockList />} />
          <Route path="/stock/:symbol" element={<StockDetail />} />
        </Routes>
      </div>
    </Router>
  );
}

export default App;
