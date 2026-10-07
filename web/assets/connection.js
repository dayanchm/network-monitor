let activeView = "connection";
let selectedDate = new Date().toLocaleDateString("en-CA");
let historyRequest = 0;
let connectionSamples = [];
async function loadConnection() {
      const request = ++historyRequest;
      count.textContent = "Connection history";
      content.className = "connection";
      content.innerHTML = `<div class="connection-toolbar"><label for="connection-date">Date</label><input id="connection-date" type="date" value="${escapeHTML(selectedDate)}"><span id="connection-status" role="status">Loading...</span></div><div id="connection-results"></div>`;
      document.querySelector("#connection-date").addEventListener("change", e => { if(e.target.value) {selectedDate=e.target.value; loadConnection();} });
      const start = new Date(selectedDate + "T00:00:00");
      const end = new Date(start); end.setDate(end.getDate()+1);
      try {
        const response = await fetch(`/api/connection-history?start=${encodeURIComponent(start.toISOString())}&end=${encodeURIComponent(end.toISOString())}`);
        if(!response.ok) throw new Error(`Request failed: ${response.status}`);
        const samples = await response.json();
        if(activeView !== "connection" || request !== historyRequest) return;
        connectionSamples = samples;
        document.querySelector("#connection-status").textContent = `${samples.length} measurements`;
        const results = document.querySelector("#connection-results");
        if(!samples.length) {results.className="empty"; results.textContent="No measurements recorded for this date."; return;}
        const colors = {healthy:"#15803d", degraded:"#ca8a04", unreachable:"#dc2626", unknown:"#94a3b8"};
        const problems = samples.filter(s => s.status === "degraded" || s.status === "unreachable");
        results.innerHTML = `<div class="connection"><div class="connection-summary"><span>${samples.filter(s=>s.status==="healthy").length} healthy</span><span>${problems.length} problem measurements</span><span>${samples.filter(s=>s.status==="unknown").length} unknown</span></div><div class="legend">${Object.entries(colors).map(([key,color])=>`<span style="--swatch:${color}">${key}</span>`).join("")}</div><div class="chart-section"><h2>Connection status</h2><canvas class="chart" id="status-chart" role="img" aria-label="Daily connection status timeline"></canvas></div><div class="chart-section"><h2>Latency (ms)</h2><canvas class="chart" id="latency-chart" role="img" aria-label="Measured latency over the day"></canvas></div><div class="chart-section"><h2>Packet loss (%)</h2><canvas class="chart" id="loss-chart" role="img" aria-label="Measured packet loss over the day"></canvas></div><div class="table-wrap"><table><thead><tr><th>Time</th><th>Status</th><th>Latency</th><th>Loss</th><th>DNS</th></tr></thead><tbody>${[...samples].reverse().map(s=>`<tr><td>${escapeHTML(formatDate(s.time))}</td><td style="color:${colors[s.status]||colors.unknown}">${escapeHTML(s.status)}</td><td>${s.result.latency_measured?Number(s.result.latency_ms).toFixed(1)+" ms":"Unknown"}</td><td>${s.result.packet_loss_measured?Number(s.result.packet_loss).toFixed(1)+"%":"Unknown"}</td><td>${s.result.dns_healthy?"Healthy":"Failed"}</td></tr>`).join("")}</tbody></table></div></div>`;
        results.className = "";
        renderProblemTimeline(samples, results);
        drawConnectionCharts(samples,start,end);
      } catch(error) { if(activeView==="connection" && request===historyRequest) {document.querySelector("#connection-status").textContent=error.message;} }
    }

    function drawConnectionCharts(samples,start,end) {
      const colors={healthy:"#15803d",degraded:"#ca8a04",unreachable:"#dc2626",unknown:"#94a3b8"};
      for(const [id,key,flag] of [["status-chart",null,null],["latency-chart","latency_ms","latency_measured"],["loss-chart","packet_loss","packet_loss_measured"]]) {
        const canvas=document.getElementById(id); if(!canvas) return;
        const width=canvas.clientWidth, height=220, ratio=devicePixelRatio||1;
        canvas.width=width*ratio; canvas.height=height*ratio;
        const ctx=canvas.getContext("2d"); ctx.scale(ratio,ratio);
        const left=42,right=width-14,top=16,bottom=185;
        const x=t=>left+(new Date(t)-start)/(end-start)*(right-left);
        const max=key==="packet_loss"?100:Math.max(100,...samples.filter(s=>s.result[flag]).map(s=>s.result[key]));
        ctx.font="12px system-ui";ctx.fillStyle="#64748b";
        for(let i=0;i<=4;i++){const y=bottom-(bottom-top)*i/4;ctx.strokeStyle="#e2e8f0";ctx.beginPath();ctx.moveTo(left,y);ctx.lineTo(right,y);ctx.stroke();if(key)ctx.fillText(String(Math.round(max*i/4)),2,y+4);}
        for(let i=0;i<=4;i++){const at=new Date(+start+(end-start)*i/4);ctx.textAlign=i===4?"right":i===0?"left":"center";ctx.fillText(at.toLocaleTimeString([], {hour:"2-digit",minute:"2-digit"}),left+(right-left)*i/4,210);}ctx.textAlign="left";
        let previous=null;
        for(const s of samples){const xx=x(s.time);ctx.fillStyle=colors[s.status]||colors.unknown;
          if(!key){const until=Math.min(+new Date(s.time)+60000,+end,Date.now());ctx.fillRect(xx,65,Math.max(1,x(until)-xx),70);continue;}
          if(!s.result[flag]){previous=null;continue;}
          const yy=bottom-s.result[key]/max*(bottom-top);
          if(previous && new Date(s.time)-new Date(previous.time)<=90000){ctx.strokeStyle=key==="packet_loss"?"#be123c":"#0f766e";ctx.beginPath();ctx.moveTo(previous.x,previous.y);ctx.lineTo(xx,yy);ctx.stroke();}
          ctx.beginPath();ctx.arc(xx,yy,2.5,0,Math.PI*2);ctx.fill();previous={x:xx,y:yy,time:s.time};
        }
        canvas.onmousemove=e=>{const fraction=(e.offsetX-left)/(right-left);const target=+start+fraction*(end-start);const s=samples.reduce((a,b)=>Math.abs(new Date(b.time)-target)<Math.abs(new Date(a.time)-target)?b:a);canvas.title=`${formatDate(s.time)}: ${s.status}${key&&s.result[flag]?" - "+s.result[key]:""}`;};
      }
    }
    window.addEventListener("resize",()=>{if(activeView==="connection" && connectionSamples.length){const start=new Date(selectedDate+"T00:00:00");const end=new Date(start);end.setDate(end.getDate()+1);drawConnectionCharts(connectionSamples,start,end);}});
    setInterval(()=>{if(activeView==="connection")loadConnection();},60000);
function renderProblemTimeline(samples, results) {
  const events = buildProblemEvents(samples);
  const section = document.createElement("section");
  section.className = "problem-timeline";
  section.setAttribute("aria-label", "Problem timeline");
  section.innerHTML = `<div class="timeline-heading"><h2>Problem timeline</h2><span>${events.length} events</span></div>${events.length ? `<ol class="event-list">${events.map(event => {
    const ending = event.state === "recovered"
      ? `Normal measurement at ${formatDate(event.recoveredAt)}`
      : event.state === "interrupted" ? "Data interrupted; recovery unknown" : "Present at last measurement; recovery not observed";
    return `<li class="event event-${event.key}"><div class="event-heading"><strong>${event.label}</strong><span class="event-state">${event.state === "recovered" ? "Recovered" : "Unconfirmed end"}</span></div><p>First observed: ${escapeHTML(formatDate(event.start))}</p><p>Last problem measurement: ${escapeHTML(formatDate(event.last))}</p><div class="event-ending">${escapeHTML(ending)}<span>${event.count} measurements</span></div></li>`;
  }).join("")}</ol>` : `<p class="timeline-empty">No measured problems in this period.</p>`}`;
  const container = results.querySelector(".connection");
  container.insertBefore(section, container.querySelector(".table-wrap"));
}

refresh.addEventListener("click", loadConnection);
document.querySelector("#analyze").addEventListener("click", async () => {
  const button = document.querySelector("#analyze");
  const output = document.querySelector("#ai-comment");
  const day = selectedDate;
  const start = new Date(day + "T00:00:00");
  const end = new Date(start); end.setDate(end.getDate()+1);
  button.disabled = true;
  output.textContent = "Analyzing...";
  try {
    const response = await fetch("/api/connection-analysis", {
      method: "POST", headers: {"Content-Type":"application/json"},
      body: JSON.stringify({start:start.toISOString(),end:end.toISOString()}),
    });
    if (!response.ok) throw new Error((await response.text()).trim());
    const data = await response.json();
    document.querySelector("#ai-meta").textContent = `${data.provider} / Measurement: ${formatDate(data.time)}`;
    output.textContent = data.analysis;
  } catch(error) { output.textContent = error.message; }
  finally { button.disabled = false; }
});
loadConnection();
