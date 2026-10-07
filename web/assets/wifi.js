function renderWifiPasswordForm() {
      count.textContent = "Wi-Fi";
      content.className = "panel";
      content.innerHTML = `
        <form class="form" id="wifi-password-form">
          <div class="field">
            <label for="router-ip">Router IP</label>
            <input id="router-ip" name="routerIp" type="text" inputmode="numeric" autocomplete="off" value="192.168.1.1" required>
          </div>
          <div class="field">
            <label for="router-username">Admin Username</label>
            <input id="router-username" name="username" type="text" autocomplete="username" value="admin" required>
          </div>
          <div class="field">
            <label for="router-admin-password">Admin Password</label>
            <input id="router-admin-password" name="admin" type="password" autocomplete="current-password" required>
          </div>
          <div class="field">
            <label for="wifi-password">New Wi-Fi Password</label>
            <input id="wifi-password" name="password" type="password" autocomplete="new-password" minlength="8" required>
          </div>
          <div class="field">
            <label for="wifi-password-confirm">Confirm Password</label>
            <input id="wifi-password-confirm" name="confirm" type="password" autocomplete="new-password" minlength="8" required>
          </div>
          <button id="save-wifi-password" type="submit">Change Password</button>
          <div class="message" id="wifi-password-message" role="status"></div>
        </form>
      `;

      document.querySelector("#wifi-password-form").addEventListener("submit", changeWifiPassword);
    }
async function changeWifiPassword(event) {
      event.preventDefault();
      const form = event.currentTarget;
      const routerIp = form.routerIp.value.trim();
      const username = form.username.value.trim();
      const admin = form.admin.value.trim();
      const password = form.password.value.trim();
      const confirm = form.confirm.value.trim();
      const button = form.querySelector("#save-wifi-password");
      const message = form.querySelector("#wifi-password-message");

      message.className = "message";
      message.textContent = "";

      if (!routerIp || !username || !admin) {
        message.className = "message error-text";
        message.textContent = "Router IP, admin username, and admin password are required.";
        return;
      }

      if (password.length < 8) {
        message.className = "message error-text";
        message.textContent = "Password must be at least 8 characters.";
        return;
      }

      if (password !== confirm) {
        message.className = "message error-text";
        message.textContent = "Passwords do not match.";
        return;
      }

      button.disabled = true;
      button.textContent = "Changing...";
      try {
        const response = await fetch("/api/wifi/password", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            router_ip: routerIp,
            username,
            admin,
            password,
          }),
        });
        if (!response.ok) {
          throw new Error(await response.text());
        }
        form.reset();
        message.className = "message ok";
        message.textContent = "Wi-Fi password changed.";
      } catch (error) {
        message.className = "message error-text";
        message.textContent = error.message.trim() || "Could not change Wi-Fi password.";
      } finally {
        button.disabled = false;
        button.textContent = "Change Password";
      }
    }
refresh.hidden = true;
renderWifiPasswordForm();

