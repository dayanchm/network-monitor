function renderSites(visits) {
      visits = visits || [];
      count.textContent = `${visits.length} ${visits.length === 1 ? "site" : "sites"}`;

      if (!visits.length) {
        content.className = "empty";
        content.textContent = "No site activity found yet.";
        return;
      }

      content.className = "table-wrap";
      content.innerHTML = `
        <table>
          <thead>
            <tr>
              <th>Device IP</th>
              <th>MAC Address</th>
              <th>Hostname</th>
              <th>Domain</th>
              <th>Count</th>
              <th>First Seen</th>
              <th>Last Seen</th>
            </tr>
          </thead>
          <tbody>
            ${visits.map((visit) => {
              const hostname = visit.hostname || "unknown";
              const hostnameClass = hostname === "unknown" ? "unknown" : "ok";
              return `
                <tr>
                  <td><code>${escapeHTML(visit.device_ip)}</code></td>
                  <td><code>${escapeHTML(visit.mac)}</code></td>
                  <td><span class="badge ${hostnameClass}">${escapeHTML(hostname)}</span></td>
                  <td><code>${escapeHTML(visit.domain)}</code></td>
                  <td>${escapeHTML(visit.count)}</td>
                  <td class="muted">${escapeHTML(formatDate(visit.first_seen))}</td>
                  <td class="muted">${escapeHTML(formatDate(visit.last_seen))}</td>
                </tr>
              `;
            }).join("")}
          </tbody>
        </table>
      `;
    }
function loadSites() { return loadData("/api/sites", renderSites); }
refresh.addEventListener("click", loadSites);
loadSites();

