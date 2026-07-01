import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { ConfigProvider, App as AntdApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import ErrorBoundary from '../src/components/ErrorBoundary';
import Screen from '../src/pages/Screen';

export default function App() {
  return (
    <ConfigProvider locale={zhCN} theme={{ token: { colorPrimary: '#006875' } }}>
      <AntdApp>
        <ErrorBoundary>
        <BrowserRouter>
          <Routes>
            <Route path="*" element={<Screen />} />
          </Routes>
        </BrowserRouter>
        </ErrorBoundary>
      </AntdApp>
    </ConfigProvider>
  );
}
