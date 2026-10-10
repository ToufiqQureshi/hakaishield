import { lazy, Suspense } from 'react';
import { BrowserRouter, Navigate, Routes, Route } from 'react-router-dom';
import { ThemeProvider } from './context/ThemeContext';
import Layout from './components/Layout';
import RequireAuth from './components/RequireAuth';
import EvidenceLogs from './pages/EvidenceLogs';
import MitigationRules from './pages/MitigationRules';
import ProtectionSettings from './pages/ProtectionSettings';
import DomainsSiem from './pages/DomainsSiem';
import Landing from './pages/Landing';
import Pricing from './pages/Pricing';
import Changelog from './pages/Changelog';
import Docs from './pages/Docs';
import Contact from './pages/Contact';
import SignIn from './pages/SignIn';
import SignUp from './pages/SignUp';
import ForgotPassword from './pages/ForgotPassword';
import ResetPassword from './pages/ResetPassword';
import Subscription from './pages/Subscription';
import Payment from './pages/Payment';
import About from './pages/About';
import Terms from './pages/Terms';
import Privacy from './pages/Privacy';
import AcceptableUse from './pages/AcceptableUse';
import Cookies from './pages/Cookies';
import DataProcessing from './pages/DataProcessing';
import CommunicationPreferences from './pages/CommunicationPreferences';

// The chart library is needed only after authentication. Keep it out of the
// public landing and signup bundle.
const Overview = lazy(() => import('./pages/Overview'));

function App() {
  return (
    <ThemeProvider>
      <BrowserRouter>
        <Routes>
          {/* Marketing Pages (no dashboard layout) */}
          <Route path="/landing" element={<Landing />} />
          <Route path="/pricing" element={<Pricing />} />
          <Route path="/changelog" element={<Changelog />} />
          <Route path="/docs" element={<Docs />} />
          <Route path="/contact" element={<Contact />} />
          <Route path="/about" element={<About />} />
          <Route path="/terms" element={<Terms />} />
          <Route path="/privacy" element={<Privacy />} />
          <Route path="/acceptable-use" element={<AcceptableUse />} />
          <Route path="/cookies" element={<Cookies />} />
          <Route path="/data-processing" element={<DataProcessing />} />

          {/* Authentication Pages */}
          <Route path="/sign-in" element={<SignIn />} />
          <Route path="/sign-up" element={<SignUp />} />
          <Route path="/login" element={<Navigate to="/sign-in" replace />} />
          <Route path="/forgot-password" element={<ForgotPassword />} />
          <Route path="/reset-password" element={<ResetPassword />} />
          <Route path="/onboarding" element={<Navigate to="/domains-siem" replace />} />

          {/* Subscription & Payment */}
          <Route path="/subscription" element={<Subscription />} />
          <Route path="/payment" element={<Payment />} />

          {/* Dashboard (requires a signed-in session) */}
          <Route element={<RequireAuth />}>
            <Route path="/" element={<Layout />}>
              <Route index element={<Suspense fallback={<p role="status" className="p-6 text-sm">Loading dashboard…</p>}><Overview /></Suspense>} />
              <Route path="evidence-logs" element={<EvidenceLogs />} />
              <Route path="mitigation-rules" element={<MitigationRules />} />
              <Route path="protection-settings" element={<ProtectionSettings />} />
              <Route path="domains-siem" element={<DomainsSiem />} />
              <Route path="communication-preferences" element={<CommunicationPreferences />} />
            </Route>
          </Route>

          {/* Unknown URLs used to render a blank page. */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </ThemeProvider>
  );
}

export default App;
