import { useEffect, useState } from 'react';
import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { signedOutPath } from '../lib/authRedirect';
import { supabase } from '../lib/supabaseClient';

// Gates the dashboard routes behind a signed-in Supabase session.
// Checks the session asynchronously (supabase-js reads it from
// storage, which isn't synchronous) rather than assuming signed-out
// during that brief check — a flash of the sign-in page would be
// worse than a brief loading state for a session that turns out valid.
export default function RequireAuth() {
  const [status, setStatus] = useState<'checking' | 'signed-in' | 'signed-out'>('checking');
  const { pathname } = useLocation();

  useEffect(() => {
    let cancelled = false;
    supabase.auth.getSession().then(({ data }) => {
      if (!cancelled) setStatus(data.session ? 'signed-in' : 'signed-out');
    });
    const { data: sub } = supabase.auth.onAuthStateChange((_event, session) => {
      setStatus(session ? 'signed-in' : 'signed-out');
    });
    return () => {
      cancelled = true;
      sub.subscription.unsubscribe();
    };
  }, []);

  if (status === 'checking') return null;
  if (status === 'signed-out') return <Navigate to={signedOutPath(pathname)} replace />;
  return <Outlet />;
}
