function buildProblemEvents(samples) {
  const rules = [
    { key: "latency", label: "High latency", value: r => r.latency_measured ? r.latency_ms >= 150 : null },
    { key: "loss", label: "Packet loss", value: r => r.packet_loss_measured ? r.packet_loss > 0 : null },
    { key: "unreachable", label: "Target unreachable", value: r => r.internet_status === "unreachable" ? true : r.internet_status === "reachable" ? false : null },
  ];
  const events = [];
  const ordered = [...samples].sort((a, b) => new Date(a.time) - new Date(b.time));
  for (const rule of rules) {
    let event = null;
    let previous = null;
    for (const sample of ordered) {
      const time = new Date(sample.time).getTime();
      if (!Number.isFinite(time)) continue;
      const value = rule.value(sample.result);
      const gap = previous !== null && time - previous > 90000;
      if (event && (gap || value === null)) {
        event.state = "interrupted";
        events.push(event);
        event = null;
      }
      if (value === true) {
        if (!event) event = { key: rule.key, label: rule.label, start: sample.time, last: sample.time, count: 0, state: "last-observed" };
        event.last = sample.time;
        event.count++;
      } else if (value === false && event) {
        event.state = "recovered";
        event.recoveredAt = sample.time;
        events.push(event);
        event = null;
      }
      previous = time;
    }
    if (event) events.push(event);
  }
  return events.sort((a, b) => new Date(b.start) - new Date(a.start));
}

if (typeof module !== "undefined") module.exports = { buildProblemEvents };
