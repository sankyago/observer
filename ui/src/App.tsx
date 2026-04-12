import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { ConfigProvider } from 'antd';

export default function App() {
  return (
    <ConfigProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<div>Loading...</div>} />
          <Route path="/flows/:id" element={<div>Loading...</div>} />
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  );
}
