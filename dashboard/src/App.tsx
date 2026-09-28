import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { ThemeProvider } from './context/ThemeContext';
import Layout from './components/Layout';
import RequireAuth from './components/RequireAuth';
import Overview from './pages/Overview';
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
import Onboarding from './pages/Onboarding';
import Subscription from './pages/Subscription';
import Payment from './pages/Payment';
import About from './pages/About';
import Terms from './pages/Terms';
import Privacy from './pages/Privacy';

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

          {/* Authentication Pages */}
          <Route path="/sign-in" element={<SignIn />} />
          <Route path="/sign-up" element={<SignUp />} />
          <Route path="/forgot-password" element={<ForgotPassword />} />
          <Route path="/onboarding" element={<Onboarding />} />

          {/* Subscription & Payment */}
          <Route path="/subscription" element={<Subscription />} />
          <Route path="/payment" element={<Payment />} />

          {/* Dashboard (requires a signed-in session) */}
          <Route element={<RequireAuth />}>
            <Route path="/" element={<Layout />}>
              <Route index element={<Overview />} />
              <Route path="evidence-logs" element={<EvidenceLogs />} />
              <Route path="mitigation-rules" element={<MitigationRules />} />
              <Route path="protection-settings" element={<ProtectionSettings />} />
              <Route path="domains-siem" element={<DomainsSiem />} />
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </ThemeProvider>
  );
}

export default App;
