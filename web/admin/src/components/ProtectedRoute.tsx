import { useEffect, useRef } from 'react';
import { Navigate, useNavigate, useLocation } from 'react-router-dom';
import { isTokenExpired } from '../utils/auth';

export default function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const location = useLocation();
  const locationRef = useRef(location);

  // Sync ref in effect, not during render (avoids React strict-mode warning).
  useEffect(() => { locationRef.current = location; }, [location]);

  const token = localStorage.getItem('token');

  useEffect(() => {
    const handler = () => {
      const loc = locationRef.current;
      navigate(`/login?redirect=${encodeURIComponent(loc.pathname + loc.search)}`, { replace: true });
    };
    window.addEventListener('auth-expired', handler);
    return () => window.removeEventListener('auth-expired', handler);
  }, [navigate]);

  if (!token || isTokenExpired(token)) {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    const redirect = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?redirect=${redirect}`} replace />;
  }
  return <>{children}</>;
}
