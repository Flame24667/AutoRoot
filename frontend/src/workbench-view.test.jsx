import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import test from 'node:test';
import assert from 'node:assert/strict';
import WorkbenchView from './WorkbenchView.jsx';

const device = { model: 'SM-A065F', serial: 'TEST-DEVICE', rooted: true, bootloaderLocked: false };
const session = { model: device.model, serial: device.serial, stage: 'rooted' };
const proof = { rooted: true, serial: device.serial, raw: 'uid=0(root) gid=0(root)' };
const render = (extra = {}) => renderToStaticMarkup(<WorkbenchView device={device} session={session} rootProof={proof} {...extra}/>);
const disabled = (html, label) => new RegExp('<button[^>]*disabled=""[^>]*>' + label + '</button>').test(html);

test('root badge requires live matching uid=0 evidence', () => {
  assert.match(render(), /ROOTED: YES/);
  assert.doesNotMatch(render({rootProof: {...proof, raw: 'uid=00(root)'}}), /ROOTED: YES/);
  assert.doesNotMatch(render({rootProof: {...proof, serial: 'OTHER'}}), /ROOTED: YES/);
  assert.doesNotMatch(render({device: null}), /ROOTED: YES/);
});
test('completed session cannot flash even with stale approval and preflight', () => {
  const html = render({activeTab: 'flash', approved: true, preflight: {ok: true}});
  assert.ok(disabled(html, 'Flash &amp; wipe data'));
  assert.ok(disabled(html, 'Approve plan'));
});
test('disconnected phone cannot flash', () => {
  const html = render({activeTab: 'flash', device: null, session: {...session, stage: 'preflight-ok'}, approved: true, preflight: {ok: true}});
  assert.ok(disabled(html, 'Flash &amp; wipe data'));
});
test('failed-session recovery is explicit and blocked for disconnected phones', () => {
  const props = {activeTab: 'logs', session: {...session, stage: 'failed'}, device: {...device, rooted: false}};
  assert.match(render(props), /Recover pre-launch failure \(no flash\)/);
  assert.ok(disabled(render({...props, device: null}), 'Recover pre-launch failure \\(no flash\\)'));
  assert.doesNotMatch(render({activeTab: 'logs'}), /Recover pre-launch failure/);
});
test('all navigation panels render with English operator labels', () => {
  for (const activeTab of ['overview', 'prepare', 'flash', 'logs']) {
    const html = render({activeTab});
    assert.match(html, /Refresh connection/);
    assert.doesNotMatch(html, /Persiapan|Ringkasan|Perbarui|Belum|Perangkat|dilakukan|Lanjutkan|otomatis/);
  }
});
test('AutoRoot branding keeps experimental scope without PoC in the displayed name', () => {
  const html = render({activeTab: 'overview'});
  assert.match(html, /EXPERIMENTAL/);
  assert.match(html, /Tested on SM-A065F only/);
  assert.doesNotMatch(html, /AutoRoot PoC|LAB \/ POC|Local PoC/);
});
test('sidebar separates fixed tested configuration from live device', () => {
  const html = render({device: {...device, model: 'OTHER-MODEL', serial: 'OTHER-SERIAL'}});
  assert.match(html, /TESTED CONFIGURATION/);
  assert.match(html, /CONNECTED DEVICE/);
  assert.match(html, /OTHER-MODEL/);
  assert.match(html, /OTHER-SERIAL/);
  assert.doesNotMatch(render({device: null}), /CONNECTED DEVICE<\/span><strong>SM-A065F/);
});
test('mismatched device and busy operations cannot flash', () => {
  for (const extra of [{device: {...device, serial: 'OTHER'}}, {busy: true}]) {
    const html = render({activeTab: 'flash', session: {...session, stage: 'preflight-ok'}, device: {...device, rooted: false}, approved: true, preflight: {ok: true}, ...extra});
    assert.ok(disabled(html, 'Flash &amp; wipe data'));
  }
});
