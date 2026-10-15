(() => {
  const $ = (s) => document.querySelector(s);
  const $$ = (s) => [...document.querySelectorAll(s)];

  let selectedDevice = null;
  let selectedFiles = [];

  // Tabs
  $$('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      $$('.tab').forEach(t => t.classList.remove('active'));
      $$('.tab-panel').forEach(p => p.classList.remove('active'));
      tab.classList.add('active');
      $(`#tab-${tab.dataset.tab}`).classList.add('active');
      if (tab.dataset.tab === 'inbox') loadInbox();
    });
  });

  // Dropzone
  const dz = $('#dropzone');
  const fileInput = $('#file-input');

  dz.addEventListener('click', () => fileInput.click());
  fileInput.addEventListener('change', (e) => {
    selectedFiles = [...e.target.files];
    updateSelectedFiles();
  });

  ;['dragenter', 'dragover'].forEach(ev => {
    dz.addEventListener(ev, (e) => {
      e.preventDefault();
      dz.classList.add('dragover');
    });
  });
  ;['dragleave', 'drop'].forEach(ev => {
    dz.addEventListener(ev, (e) => {
      e.preventDefault();
      dz.classList.remove('dragover');
    });
  });
  dz.addEventListener('drop', (e) => {
    selectedFiles = [...e.dataTransfer.files];
    updateSelectedFiles();
  });

  function updateSelectedFiles() {
    const el = $('#selected-files');
    if (selectedFiles.length === 0) {
      el.hidden = true;
      $('#send-btn').disabled = true;
      return;
    }
    el.hidden = false;
    el.textContent = `${selectedFiles.length} file(s) selected: ${selectedFiles.map(f => f.name).join(', ')}`;
    $('#send-btn').disabled = !selectedDevice;
  }

  // Devices
  async function loadDevices() {
    try {
      const res = await fetch('/api/devices');
      const devices = await res.json();
      const list = $('#device-list');
      list.innerHTML = '';

      if (!Array.isArray(devices) || devices.length === 0) {
        list.innerHTML = '<p class="empty">No devices found</p>';
        return;
      }

      devices.forEach((d, i) => {
        const available = d.taildrop === 'available';
        const card = document.createElement('div');
        card.className = `device-card ${available ? '' : 'disabled'}`;
        card.style.animationDelay = `${i * 40}ms`;
        card.innerHTML = `
          <div class="device-icon">${osIcon(d.os)}</div>
          <div class="device-info">
            <div class="device-name">${esc(d.name)}</div>
            <div class="device-meta">${d.os || 'unknown'} · ${d.online ? 'online' : 'offline'}</div>
          </div>
          <span class="tag ${available ? 'ok' : ''}">${available ? 'ready' : esc(d.taildrop)}</span>
        `;
        if (available) {
          card.addEventListener('click', () => {
            $$('.device-card').forEach(c => c.classList.remove('selected'));
            card.classList.add('selected');
            selectedDevice = d.name;
            $('#send-btn').disabled = selectedFiles.length === 0;
          });
        }
        list.appendChild(card);
      });
    } catch (err) {
      toast('Failed to load devices: ' + err.message, 'error');
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

  // Inbox
  async function loadInbox() {
    try {
      const res = await fetch('/api/inbox');
      const files = await res.json();
      const list = $('#inbox-list');
      const badge = $('#inbox-badge');

      if (!Array.isArray(files) || files.length === 0) {
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
        card.style.animationDelay = `${i * 40}ms`;
        card.innerHTML = `
          <div class="file-icon">📄</div>
          <div class="file-info">
            <div class="file-name">${esc(f.name)}</div>
            <div class="file-meta">${formatSize(f.size)}</div>
          </div>
          <button class="btn small success" data-save="${esc(f.name)}">Save</button>
          <button class="btn small danger" data-del="${esc(f.name)}">Delete</button>
        `;
        list.appendChild(card);
      });

      list.querySelectorAll('[data-save]').forEach(btn => {
        btn.addEventListener('click', async () => {
          btn.disabled = true;
          btn.textContent = '…';
          try {
            const res = await fetch('/api/inbox/save', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ name: btn.dataset.save })
            });
            const data = await res.json();
            if (data.error) throw new Error(data.error);
            toast(`Saved to ${data.path}`, 'success');
            loadInbox();
          } catch (e) {
            toast(e.message, 'error');
            btn.disabled = false;
            btn.textContent = 'Save';
          }
        });
      });

      list.querySelectorAll('[data-del]').forEach(btn => {
        btn.addEventListener('click', async () => {
          try {
            await fetch('/api/inbox/delete', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ name: btn.dataset.del })
            });
            toast('Deleted', 'success');
            loadInbox();
          } catch (e) {
            toast(e.message, 'error');
          }
        });
      });
    } catch (err) {
      toast('Failed to load inbox: ' + err.message, 'error');
    }
  }

  $('#refresh-inbox').addEventListener('click', loadInbox);

  // Send
  $('#send-btn').addEventListener('click', async () => {
    if (!selectedDevice || selectedFiles.length === 0) return;
    const btn = $('#send-btn');
    btn.disabled = true;
    btn.textContent = 'Sending…';

    const fd = new FormData();
    fd.append('device', selectedDevice);
    selectedFiles.forEach(f => fd.append('files', f));

    try {
      const res = await fetch('/api/send', { method: 'POST', body: fd });
      const results = await res.json();
      results.forEach(r => {
        if (r.error) toast(`${r.name}: ${r.error}`, 'error');
        else toast(`${r.name} sent`, 'success');
      });
      selectedFiles = [];
      fileInput.value = '';
      updateSelectedFiles();
    } catch (e) {
      toast(e.message, 'error');
    } finally {
      btn.disabled = false;
      btn.textContent = 'Send to selected';
    }
  });

  // Status
  async function loadStatus() {
    try {
      const res = await fetch('/api/status');
      const st = await res.json();
      const el = $('#status');
      const text = $('#status-text');
      if (st.error) {
        el.className = 'status error';
        text.textContent = 'Offline';
      } else {
        el.className = 'status online';
        text.textContent = st.backendState || 'Connected';
        $('#self-name').textContent = st.self || '';
        $('#ver').textContent = st.version || '0.1.0';
      }
    } catch {
      $('#status').className = 'status error';
      $('#status-text').textContent = 'Daemon unreachable';
    }
  }

  // Helpers
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
      t.style.transform = 'translateY(8px)';
      t.style.transition = '0.25s';
      setTimeout(() => t.remove(), 250);
    }, 3200);
  }

  // Init
  loadStatus();
  loadDevices();
  loadInbox();
  setInterval(loadStatus, 15000);
  setInterval(loadDevices, 20000);
  setInterval(loadInbox, 8000);
})();
