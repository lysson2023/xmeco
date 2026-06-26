import { useEffect } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { isTokenExpired } from '../utils/auth';

export default function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const token = localStorage.getItem('token');

  useEffect(() => {
    const handler = () => navigate('/login', { replace: true });
    window.addEventListener('auth-expired', handler);
    return () => window.removeEventListener('auth-expired', handler);
  }, [navigate]);

  if (!token || isTokenExpired(token)) {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}
