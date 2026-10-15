(() => {
  const $ = (s) => document.querySelector(s);
  const $$ = (s) => [...document.querySelectorAll(s)];

  // Wails bindings (generated at runtime)
  const App = () => window.go?.main?.App;

  let selectedDevice = null;
  let selectedPaths = []; // absolute paths from dialog

  $$('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      $$('.tab').forEach(t => t.classList.remove('active'));
      $$('.tab-panel').forEach(p => p.classList.remove('active'));
      tab.classList.add('active');
      $(`#tab-${tab.dataset.tab}`).classList.add('active');
      if (tab.dataset.tab === 'inbox') loadInbox();
    });
  });

  const dz = $('#dropzone');
  dz.addEventListener('click', async () => {
    try {
      // Native file dialog via Wails runtime
      if (window.runtime?.OpenMultipleFilesDialog) {
        const files = await window.runtime.OpenMultipleFilesDialog({
          title: 'Select files to send',
        });
        if (files && files.length) {
          selectedPaths = files;
          updateSelected();
        }
      } else {
        // Fallback for browser dev
        $('#file-input').click();
      }
    } catch (e) {
      toast(String(e), 'error');
    }
  });

  $('#file-input')?.addEventListener('change', (e) => {
    // Browser fallback — no real paths, can't send via CLI
    selectedPaths = [...e.target.files].map(f => f.name);
    updateSelected();
    toast('In desktop app use the native dialog for real paths', 'error');
  });

  function updateSelected() {
    const el = $('#selected-files');
    if (!selectedPaths.length) {
      el.hidden = true;
      $('#send-btn').disabled = true;
      return;
    }
    el.hidden = false;
    el.textContent = `${selectedPaths.length} file(s): ${selectedPaths.map(p => p.split(/[/\\]/).pop()).join(', ')}`;
    $('#send-btn').disabled = !selectedDevice;
  }

  async function loadDevices() {
    try {
      const app = App();
      if (!app) return;
      const devices = await app.GetDevices();
      const list = $('#device-list');
      list.innerHTML = '';
      if (!devices?.length) {
        list.innerHTML = '<p class="empty">No devices found</p>';
        return;
      }
      devices.forEach((d, i) => {
        const available = d.taildrop === 'available';
        const card = document.createElement('div');
        card.className = `device-card ${available ? '' : 'disabled'}`;
        card.style.animationDelay = `${i * 35}ms`;
        card.innerHTML = `
          <div class="device-icon">${osIcon(d.os)}</div>
          <div class="device-info">
            <div class="device-name">${esc(d.name)}</div>
            <div class="device-meta">${d.os || '?'} · ${d.online ? 'online' : 'offline'}</div>
          </div>
          <span class="tag ${available ? 'ok' : ''}">${available ? 'ready' : esc(d.taildrop)}</span>`;
        if (available) {
          card.onclick = () => {
            $$('.device-card').forEach(c => c.classList.remove('selected'));
            card.classList.add('selected');
            selectedDevice = d.name;
            $('#send-btn').disabled = !selectedPaths.length;
          };
        }
        list.appendChild(card);
      });
    } catch (e) {
      toast('Devices: ' + e, 'error');
    }
  }

  function osIcon(os) {
    if (!os) return '💻';
    const o = os.toLowerCase();
    if (o.includes('mac') || o.includes('darwin') || o.includes('ios')) return '🍎';
    if (o.includes('win')) return '🪟';
    if (o.includes('android')) return '🤖';
    if (o.includes('linux')) return '🐧';
    return '💻';
  }

  async function loadInbox() {
    try {
      const app = App();
      if (!app) return;
      const files = await app.GetInbox();
      const list = $('#inbox-list');
      const badge = $('#inbox-badge');
      if (!files?.length) {
        list.innerHTML = '<p class="empty">No files waiting</p>';
        badge.hidden = true;
        return;
      }
      badge.hidden = false;
      badge.textContent = files.length;
      list.innerHTML = '';
      files.forEach((f, i) => {
        const card = document.createElement('div');
        card.className = 'file-card';
        card.style.animationDelay = `${i * 35}ms`;
        card.innerHTML = `
          <div class="file-icon">📄</div>
          <div class="file-info">
            <div class="file-name">${esc(f.name)}</div>
            <div class="file-meta">${formatSize(f.size)}</div>
          </div>
          <button class="btn small success" data-save="${esc(f.name)}">Save</button>
          <button class="btn small danger" data-del="${esc(f.name)}">Delete</button>`;
        list.appendChild(card);
      });
      list.querySelectorAll('[data-save]').forEach(btn => {
        btn.onclick = async () => {
          btn.disabled = true;
          try {
            const path = await App().SaveFile(btn.dataset.save);
            toast('Saved: ' + path, 'success');
            loadInbox();
          } catch (e) {
            toast(String(e), 'error');
            btn.disabled = false;
          }
        };
      });
      list.querySelectorAll('[data-del]').forEach(btn => {
        btn.onclick = async () => {
          try {
            await App().DeleteFile(btn.dataset.del);
            toast('Deleted', 'success');
            loadInbox();
          } catch (e) {
            toast(String(e), 'error');
          }
        };
      });
    } catch (e) {
      toast('Inbox: ' + e, 'error');
    }
  }

  $('#refresh-inbox').onclick = loadInbox;

  $('#send-btn').onclick = async () => {
    if (!selectedDevice || !selectedPaths.length) return;
    const btn = $('#send-btn');
    btn.disabled = true;
    btn.textContent = 'Sending…';
    try {
      const results = await App().SendFiles(selectedDevice, selectedPaths);
      results.forEach(r => {
        if (r.error) toast(`${r.name}: ${r.error}`, 'error');
        else toast(`${r.name} sent`, 'success');
      });
      selectedPaths = [];
      updateSelected();
    } catch (e) {
      toast(String(e), 'error');
    } finally {
      btn.disabled = false;
      btn.textContent = 'Send to selected';
    }
  };

  async function loadStatus() {
    try {
      const app = App();
      if (!app) {
        $('#status').className = 'status error';
        $('#status-text').textContent = 'No backend';
        return;
      }
      const st = await app.GetStatus();
      if (st.error) {
        $('#status').className = 'status error';
        $('#status-text').textContent = 'Offline';
      } else {
        $('#status').className = 'status online';
        $('#status-text').textContent = st.backendState || 'Connected';
        $('#self-name').textContent = st.self || '';
        $('#ver').textContent = st.version || '0.1.0';
      }
    } catch {
      $('#status').className = 'status error';
      $('#status-text').textContent = 'Error';
    }
  }

  function formatSize(n) {
    if (n < 1024) return n + ' B';
    if (n < 1048576) return (n / 1024).toFixed(1) + ' KB';
    if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB';
    return (n / 1073741824).toFixed(2) + ' GB';
  }
  function esc(s) {
    const d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }
  function toast(msg, type = '') {
    const t = document.createElement('div');
    t.className = `toast ${type}`;
    t.textContent = msg;
    $('#toasts').appendChild(t);
    setTimeout(() => {
      t.style.opacity = '0';
      t.style.transition = '0.25s';
      setTimeout(() => t.remove(), 250);
    }, 3000);
  }

  // Wait for Wails runtime
  function boot() {
    if (!App()) {
      setTimeout(boot, 50);
      return;
    }
    loadStatus();
    loadDevices();
    loadInbox();
    setInterval(loadStatus, 12000);
    setInterval(loadDevices, 18000);
    setInterval(loadInbox, 7000);
  }
  boot();
})();
