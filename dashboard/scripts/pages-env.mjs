// Vite embeds VITE_* values into public JavaScript at build time. A Pages
// deployment with missing values can serve successfully but fail on every
// authenticated route, so check the public configuration before building.
function httpsURL(value) {
  try {
    const url = new URL(value);
    if (url.protocol !== 'https:' || url.username || url.password || url.search || url.hash) return null;
    return url;
  } catch {
    return null;
  }
}

function isServerKey(key) {
  if (key.startsWith('sb_secret_')) return true;
  const parts = key.split('.');
  if (parts.length !== 3) return false;
  try {
    return JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8')).role === 'service_role';
  } catch {
    return false;
  }
}

export function validatePagesEnv(env) {
  const errors = [];
  const supabase = httpsURL(env.VITE_SUPABASE_URL);
  if (!supabase || supabase.pathname !== '/' || supabase.hostname === 'your-project-ref.supabase.co') {
    errors.push('VITE_SUPABASE_URL must be the HTTPS Supabase project origin.');
  }

  const anonKey = env.VITE_SUPABASE_ANON_KEY?.trim();
  if (!anonKey || anonKey === 'your-anon-key' || anonKey.length < 20 || isServerKey(anonKey)) {
    errors.push('VITE_SUPABASE_ANON_KEY must be the public Supabase anon/publishable key.');
  }

  const api = httpsURL(env.VITE_API_BASE_URL);
  if (!api || api.pathname !== '/api/v1' || api.hostname.includes('localhost')) {
    errors.push('VITE_API_BASE_URL must be the HTTPS backend URL ending in /api/v1 (without trailing slash).');
  }
  return errors;
}
