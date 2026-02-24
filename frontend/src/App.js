import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import StockList from './components/StockList';
import StockDetail from './components/StockDetail';
import Portfolio from './components/Portfolio';
import './App.css';

function App() {
  return (
    <Router>
      <div className="App">
        <Routes>
          <Route path="/" element={<StockList />} />
          <Route path="/stock/:symbol" element={<StockDetail />} />
          <Route path="/portfolio" element={<Portfolio />} />
        </Routes>
      </div>
    </Router>
  );
}

export default App;
