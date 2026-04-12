import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import FlowsListPage from './pages/FlowsListPage';
import FlowEditorPage from './pages/FlowEditorPage';

export default function App() {
  return (
    <ConfigProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<FlowsListPage />} />
          <Route path="/flows/:id" element={<FlowEditorPage />} />
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  );
}
