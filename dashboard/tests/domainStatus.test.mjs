import assert from 'node:assert/strict';
import test from 'node:test';
import { domainState } from '../src/lib/domainStatus.ts';

test('only an active domain is presented as protected', () => {
  for (const status of ['pending_verification', 'verified', 'failed', '', 'ACTIVE']) {
    const state = domainState(status);
    assert.equal(state.protected, false, status);
    assert.match(state.detail, /not routing|not confirmed/);
  }
  assert.equal(domainState('active').protected, true);
});

test('each setup status tells the user the next step', () => {
  assert.match(domainState('pending_verification').detail, /DNS TXT/);
  assert.equal(domainState('verified').label, 'Ownership verified');
});
