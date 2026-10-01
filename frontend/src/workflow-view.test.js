import test from 'node:test';
import assert from 'node:assert/strict';
import { stageRank, isPostFlash, canPrepare, hasRootProof, rootLabel } from './workflow-view.js';

const device = {serial: 'phone', model: 'SM-A065F', rooted: false};
test('post-flash stages keep completed milestones visible', () => {
  for (const stage of ['waiting-boot', 'verifying', 'rooted']) {
    assert.ok(stageRank(stage) > stageRank('flash-approved'));
    assert.equal(isPostFlash({stage}), true);
    assert.equal(canPrepare({...device, stage}, device), false);
  }
  assert.equal(stageRank('failed'), -1);
});
test('preparation requires same connected unrooted phone', () => {
  const session = {...device, stage: 'firmware-valid'};
  assert.equal(canPrepare(session, device), true);
  assert.equal(canPrepare(session, {...device, rooted: true}), false);
  assert.equal(canPrepare(session, {...device, serial: 'other'}), false);
  assert.equal(canPrepare(session, null), false);
});
test('root display requires fresh matching uid=0 evidence, not saved rooted stage', () => {
  const proof = {serial: 'phone', rooted: true, raw: 'uid=0(root) gid=0(root)'};
  assert.equal(rootLabel(device, proof), 'Yes — uid=0 verified');
  assert.match(rootLabel(null, proof), /Unknown/);
  assert.match(rootLabel({...device, serial: 'other'}, proof), /Not verified/);
  assert.equal(hasRootProof({rooted: true, raw: 'uid=00(root)'}), false);
  assert.equal(hasRootProof({rooted: false, raw: proof.raw}), false);
  assert.equal(hasRootProof({rooted: true}), false);
});
