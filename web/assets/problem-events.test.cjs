const { test } = require("node:test");
const assert = require("node:assert/strict");
const { buildProblemEvents } = require("./problem-events.js");
const sample = (minute, latency, measured = true) => ({ time: new Date(Date.UTC(2026, 9, 7, 12, minute)).toISOString(), result: { latency_measured: measured, latency_ms: latency, internet_status: "reachable", packet_loss_measured: true, packet_loss: 0 } });
test("consecutive problems close on measured recovery", () => {
  const [event] = buildProblemEvents([sample(0,180),sample(1,200),sample(2,20)]);
  assert.equal(event.count,2);
  assert.equal(event.state,"recovered");
  assert.equal(event.recoveredAt,sample(2,20).time);
});
test("gaps and unavailable metrics do not imply recovery", () => {
  for (const next of [sample(3,20),sample(1,0,false)]) {
    assert.equal(buildProblemEvents([sample(0,180),next])[0].state,"interrupted");
  }
});
test("last problematic sample has no confirmed end", () => {
  assert.equal(buildProblemEvents([sample(0,180)])[0].state,"last-observed");
});
test("simultaneous problems are separate events", () => {
  const s=sample(0,180); s.result.packet_loss=25; s.result.internet_status="unreachable";
  assert.equal(buildProblemEvents([s]).length,3);
  assert.deepEqual(buildProblemEvents([sample(0,20)]),[]);
});
