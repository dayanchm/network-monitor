function loadHistory() { return loadData("/api/known-devices", data => renderDevices(data, "history")); }
refresh.addEventListener("click", loadHistory);
loadHistory();

