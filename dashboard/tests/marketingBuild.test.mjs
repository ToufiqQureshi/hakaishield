import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const dashboard = fileURLToPath(new URL('../', import.meta.url));
const vite = fileURLToPath(new URL('../node_modules/vite/bin/vite.js', import.meta.url));

test('marketing Pages build works without backend or Supabase config', () => {
  const output = mkdtempSync(join(tmpdir(), 'hakaishield-marketing-'));
  try {
    const env = { ...process.env };
    // Vite also loads dashboard/.env in local workspaces. Empty process values
    // override it so this really simulates a fresh Pages environment.
    env.VITE_API_BASE_URL = '';
    env.VITE_SUPABASE_URL = '';
    env.VITE_SUPABASE_ANON_KEY = '';
    const build = spawnSync(process.execPath, [vite, 'build', '--mode', 'marketing', '--outDir', output], {
      cwd: dashboard,
      env,
      encoding: 'utf8',
    });
    assert.equal(build.status, 0, build.stderr || build.stdout);
    assert.match(readFileSync(join(output, 'index.html'), 'utf8'), /assets\/index-/);
    const jsFiles = readdirSync(join(output, 'assets')).filter((name) => name.endsWith('.js'));
    assert.ok(jsFiles.length > 0, 'Vite must emit a JavaScript bundle');
    const scripts = jsFiles
      .map((name) => readFileSync(join(output, 'assets', name), 'utf8'))
      .join('\n');
    assert.match(scripts, /See automated traffic/);
    assert.equal(
      /VITE_API_BASE_URL|VITE_SUPABASE_ANON_KEY|sb_publishable_|supabase\.co|must be set for production dashboard builds/.test(scripts),
      false,
      'marketing bundle must not include dashboard configuration or auth code',
    );
  } finally {
    rmSync(output, { recursive: true, force: true });
  }
});
