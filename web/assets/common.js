const content = document.querySelector("#content");
const count = document.querySelector("#count");
const refresh = document.querySelector("#refresh");
function escapeHTML(value) {
      return String(value ?? "")
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#039;");
    }

    function formatDate(value) {
      if (!value) {
        return "";
      }
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) {
        return value;
      }
      return date.toLocaleString();
    }
function renderDevices(devices, mode) {
      count.textContent = `${devices.length} ${devices.length === 1 ? "device" : "devices"}`;

      if (!devices.length) {
        content.className = "empty";
        content.textContent = "No devices found.";
        return;
      }

      const dateHeaders = mode === "history" ? "<th>First Seen</th><th>Last Seen</th>" : "";
      content.className = "table-wrap";
      content.innerHTML = `
        <table>
          <thead>
            <tr>
              <th>IP Address</th>
              <th>MAC Address</th>
              <th>Hostname</th>
              <th>Status</th>
              ${dateHeaders}
            </tr>
          </thead>
          <tbody>
            ${devices.map((device) => {
              const hostname = device.hostname || "unknown";
              const hostnameClass = hostname === "unknown" ? "unknown" : "ok";
              return `
                <tr>
                  <td><code>${escapeHTML(device.ip)}</code></td>
                  <td><code>${escapeHTML(device.mac)}</code></td>
                  <td><span class="badge ${hostnameClass}">${escapeHTML(hostname)}</span></td>
                  <td><span class="badge ${device.connected ? "ok" : "unknown"}">${device.connected ? "Seen in scan" : "Not seen in scan"}</span></td>
                  ${mode === "history" ? `
                    <td class="muted">${escapeHTML(formatDate(device.first_seen))}</td>
                    <td class="muted">${escapeHTML(formatDate(device.last_seen))}</td>
                  ` : ""}
                </tr>
              `;
            }).join("")}
          </tbody>
        </table>
      `;
    }
async function loadData(endpoint, render) {
  if (refresh.disabled) return;
  refresh.disabled = true;
  refresh.textContent = "Loading...";
  try {
    const response = await fetch(endpoint, {cache: "no-store"});
    if (!response.ok) throw new Error((await response.text()).trim() || `Request failed: ${response.status}`);
    render(await response.json());
  } catch (error) {
    content.className = "error";
    content.textContent = error.message;
  } finally {
    refresh.disabled = false;
    refresh.textContent = "Refresh";
  }
}

