import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, App as AntdApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import LoginPage from './pages/Login';
import MainLayout from './layouts/Main';
import Projects from './pages/Projects';
import Buildings from './pages/Buildings';
import Devices from './pages/Devices';
import Properties from './pages/Properties';
import Registers from './pages/Registers';
import Alarms from './pages/Alarms';
import Logs from './pages/Logs';
import StartupPlans from './pages/StartupPlans';
import Users from './pages/Users';
import Agents from './pages/Agents';
import Permissions from './pages/Permissions';
import MultiAgent from './pages/MultiAgent';
import Dashboard from './pages/Screen';
import ProtectedRoute from './components/ProtectedRoute';

function App() {
  return (
    <ConfigProvider locale={zhCN} theme={{ token: { colorPrimary: '#006875' } }}>
      <AntdApp>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/" element={<ProtectedRoute><MainLayout /></ProtectedRoute>}>
              <Route index element={<Navigate to="/dashboard" replace />} />
              <Route path="dashboard" element={<Dashboard />} />
              <Route path="projects" element={<Projects />} />
              <Route path="buildings" element={<Buildings />} />
              <Route path="devices" element={<Devices />} />
              <Route path="properties" element={<Properties />} />
              <Route path="registers" element={<Registers />} />
              <Route path="alarms" element={<Alarms />} />
              <Route path="logs" element={<Logs />} />
              <Route path="startup-plans" element={<StartupPlans />} />
              <Route path="users" element={<Users />} />
              <Route path="agents" element={<Agents />} />
              <Route path="permissions" element={<Permissions />} />
              <Route path="multi-agent" element={<MultiAgent />} />
            </Route>
            <Route path="*" element={<Navigate to="/" />} />
          </Routes>
        </BrowserRouter>
      </AntdApp>
    </ConfigProvider>
  );
}

export default App;
