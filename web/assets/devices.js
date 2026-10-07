function loadDevices() { return loadData("/api/devices", data => renderDevices(data, "current")); }
refresh.addEventListener("click", loadDevices);
setInterval(loadDevices, 5000);
loadDevices();

