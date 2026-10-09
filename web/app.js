// Tab Logic
let currentTab = 'proxies';

function switchTab(tabName) {
    currentTab = tabName;
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

    if (window.event && window.event.currentTarget) {
        window.event.currentTarget.classList.add('active');
    }
    document.getElementById('tab-' + tabName).classList.add('active');
}

const btnConnect = document.getElementById('btnConnect');
const btnDisconnect = document.getElementById('btnDisconnect');
const btnAuth = document.getElementById('btnAuth');
const statusCard = document.getElementById('statusCard');
const statusText = document.getElementById('statusText');

setInterval(() => {
    updateStatus();
    fetchTunnels(false); // background refresh for tunnels state
}, 2000);

async function updateStatus() {
    try {
        const res = await fetch('/api/status');
        const data = await res.json();
        if (data.connected) {
            setUIState('connected');
        } else if (statusCard.classList.contains('connected')) {
            setUIState('disconnected');
        }
    } catch (e) {
        console.error("Error fetching status", e);
    }
}

function setUIState(state, authUrl = null) {
    statusCard.className = `glass-card status-card ${state}`;
    btnConnect.classList.remove('loading');
    btnDisconnect.classList.remove('loading');

    btnConnect.style.display = 'none';
    btnDisconnect.style.display = 'none';
    btnAuth.style.display = 'none';

    if (state === 'connected') {
        statusText.innerText = 'VPN Connected';
        btnDisconnect.style.display = 'flex';
    } else if (state === 'connecting') {
        statusText.innerText = 'Starting...';
        btnConnect.style.display = 'flex';
        btnConnect.classList.add('loading');
    } else if (state === 'auth_required') {
        statusText.innerText = 'Authentication Required';
        btnAuth.href = authUrl;
        btnAuth.style.display = 'flex';
        statusCard.className = `glass-card status-card connecting`;
    } else {
        statusText.innerText = 'Disconnected';
        btnConnect.style.display = 'flex';
    }
}

async function startVPN() {
    if (btnConnect.classList.contains('loading')) return;
    setUIState('connecting');

    try {
        const res = await fetch('/api/start', { method: 'POST' });
        const data = await res.json();

        if (data.url) {
            setUIState('auth_required', data.url);
        } else if (data.status === 'connected') {
            setUIState('connected');
        } else {
            alert('Error: ' + (data.error || 'Could not start'));
            setUIState('disconnected');
        }
    } catch (e) {
        alert('Backend connection failed.');
        setUIState('disconnected');
    }
}

async function stopVPN() {
    if (btnDisconnect.classList.contains('loading')) return;
    btnDisconnect.classList.add('loading');
    try {
        await fetch('/api/stop', { method: 'POST' });
    } catch (e) { }
}

// ====================
// PROXIES LOGIC
// ====================
async function fetchProxies() {
    try {
        const res = await fetch('/api/proxies');
        const rules = await res.json();
        const list = document.getElementById('proxyList');
        list.innerHTML = '';

        if (rules && rules.length > 0) {
            rules.forEach(rule => {
                let dotClass = rule.status === 'active' ? 'active' : '';
                list.innerHTML += `
                            <div class="proxy-card">
                                <div class="proxy-info">
                                    <div class="proxy-port"><span class="status-dot ${dotClass}"></span> Port ${rule.port}</div>
                                    <div class="proxy-target">${rule.target}</div>
                                </div>
                                <div class="proxy-actions">
                                    <button class="icon-btn" onclick="editProxy(${rule.port}, '${rule.target}')" title="Edit">
                                        <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path></svg>
                                    </button>
                                    <button class="icon-btn delete" onclick="deleteProxy(${rule.port})" title="Delete">
                                        <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                    </button>
                                </div>
                            </div>
                        `;
            });
        } else {
            list.innerHTML = '<div style="text-align: center; color: var(--text-muted); padding: 20px;">No forwarded ports active</div>';
        }
    } catch (e) { }
}

async function addProxy() {
    const portInput = document.getElementById('proxyPort');
    const targetInput = document.getElementById('proxyTarget');
    const port = parseInt(portInput.value);
    let target = targetInput.value.trim();

    if (!port || !target) {
        alert("Please provide both port and target URL.");
        return;
    }
    if (!target.startsWith('http://') && !target.startsWith('https://')) {
        target = 'http://' + target;
    }

    try {
        const res = await fetch('/api/proxies', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ port, target })
        });

        if (res.ok) {
            portInput.value = '';
            targetInput.value = '';
            document.getElementById('btnSubmitText').innerText = 'Add';
            fetchProxies();
        } else {
            const err = await res.text();
            alert("Error saving proxy: " + err);
        }
    } catch (e) {
        alert("Error connecting to server.");
    }
}

function editProxy(port, target) {
    document.getElementById('proxyPort').value = port;
    document.getElementById('proxyTarget').value = target;
    document.getElementById('btnSubmitText').innerText = 'Save';
    document.getElementById('proxyTarget').focus();
}

async function deleteProxy(port) {
    try {
        const res = await fetch('/api/proxies?port=' + port, { method: 'DELETE' });
        if (res.ok) fetchProxies();
    } catch (e) { }
}


// ====================
// TUNNELS LOGIC
// ====================
let tunnelsData = [];

async function fetchTunnels(showLoading = true) {
    if (showLoading) {
        document.getElementById('tunnelList').innerHTML = '<div style="text-align: center; color: var(--text-muted); padding: 20px;">Loading...</div>';
    }

    try {
        const res = await fetch('/api/tunnels');
        const data = await res.json();
        tunnelsData = Array.isArray(data) ? data : [];
        renderTunnels();
    } catch (e) {
        tunnelsData = [];
    }
}

function renderTunnels() {
    const list = document.getElementById('tunnelList');
    list.innerHTML = '';

    if (tunnelsData && tunnelsData.length > 0) {
        tunnelsData.forEach(t => {
            let dotClass = '';
            if (t.status === 'connected') dotClass = 'active';
            else if (t.status === 'connecting' || t.status === 'authenticating') dotClass = 'connecting';
            else if (t.status === 'needs_login') dotClass = 'connecting';
            else if (t.status && t.status.startsWith('error')) dotClass = 'error';

            let ssoBadge = '';
            let ssoBox = '';
            let actBtn = '';

            // SSO Badges and Action Buttons
            if (t.type === 'aws-ssm' && t.is_sso) {
                if (t.sso_authenticated) {
                    ssoBadge = `<span class="sso-badge ok">SSO Auth OK</span>`;
                } else {
                    ssoBadge = `<span class="sso-badge expired">SSO Requerido</span>`;
                }
            }

            if (t.status === 'authenticating') {
                actBtn = `<button class="icon-btn sso-auth" disabled style="opacity:0.8;"><svg width="14" height="14" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg><span>Esperando...</span></button>`;
                if (t.sso_auth_url) {
                    let openUrl = t.sso_auth_url;
                    if (t.sso_user_code && !openUrl.includes('user_code=')) {
                        openUrl += (openUrl.includes('?') ? '&' : '?') + 'user_code=' + encodeURIComponent(t.sso_user_code);
                    }

                    ssoBox = `
                                <div class="sso-auth-box">
                                    <div><strong>Autorización requerida:</strong> Confirma el inicio de sesión en AWS.</div>
                                    <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
                                        <a href="${openUrl}" target="_blank" rel="noopener noreferrer">Abrir enlace de verificación ↗</a>
                                        ${t.sso_user_code ? `<span class="sso-code-badge" onclick="navigator.clipboard.writeText('${t.sso_user_code}');" title="Clic para copiar">Código: ${t.sso_user_code} 📋</span>` : ''}
                                    </div>
                                    <div style="font-size:11px; color:var(--text-muted);">El código se rellena automáticamente en el navegador. Cuando confirmes, el túnel quedará listo.</div>
                                </div>
                            `;
                }
            } else if (t.type === 'aws-ssm' && t.is_sso && !t.sso_authenticated) {
                // SSO Needs login button - identical to connecting VPN
                actBtn = `<button class="icon-btn sso-auth" onclick="triggerSSOLogin('${t.id}')" title="Autenticar AWS SSO"><svg width="14" height="14" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"></path></svg><span>Autenticar SSO</span></button>`;
            } else if (t.status === 'connected' || t.status === 'connecting') {
                actBtn = `<button class="icon-btn delete" onclick="toggleTunnelStatus('${t.id}', 'stop')" title="Stop"><svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10h6v4H9z"></path></svg></button>`;
            } else {
                actBtn = `<button class="icon-btn start" onclick="toggleTunnelStatus('${t.id}', 'start')" title="Start"><svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg></button>`;
            }

            let errTitle = (t.status && t.status.startsWith('error')) ? `title="${t.status}"` : "";

            list.innerHTML += `
                        <div class="proxy-card" style="flex-direction:column; align-items:stretch;">
                            <div style="display:flex; justify-content:space-between; align-items:center;">
                                <div class="proxy-info">
                                    <div class="tunnel-name" ${errTitle}>
                                        <span class="status-dot ${dotClass}"></span> 
                                        ${t.name}
                                        <span class="tunnel-type">${t.type.toUpperCase()}</span>
                                        ${ssoBadge}
                                    </div>
                                    <div class="proxy-target">Local: ${t.local_port} → Remote: ${t.remote_host}:${t.remote_port} ${t.target ? `(Target: ${t.target})` : ''}</div>
                                </div>
                                <div class="proxy-actions">
                                    ${actBtn}
                                    <button class="icon-btn" onclick="editTunnel('${t.id}')" title="Edit">
                                        <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path></svg>
                                    </button>
                                    <button class="icon-btn delete" onclick="deleteTunnel('${t.id}')" title="Delete">
                                        <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                    </button>
                                </div>
                            </div>
                            ${ssoBox}
                        </div>
                    `;
        });
    } else {
        list.innerHTML = '<div style="text-align: center; color: var(--text-muted); padding: 20px;">No tunnels configured</div>';
    }
}

async function triggerSSOLogin(id) {
    try {
        const res = await fetch(`/api/tunnels/${id}/sso-login`, { method: 'POST' });
        if (res.ok) {
            const data = await res.json();
            if (data.auth_url) {
                let openUrl = data.auth_url;
                if (data.user_code && !openUrl.includes('user_code=')) {
                    openUrl += (openUrl.includes('?') ? '&' : '?') + 'user_code=' + encodeURIComponent(data.user_code);
                }
                window.open(openUrl, '_blank');
            }
            fetchTunnels(false);
        } else {
            const err = await res.text();
            alert("Error starting SSO login: " + err);
        }
    } catch (e) {
        alert("Error connecting to server.");
    }
}

async function toggleTunnelStatus(id, action) {
    try {
        const res = await fetch(`/api/tunnels/${id}/${action}`, { method: 'POST' });
        if (!res.ok) {
            const err = await res.text();
            alert("Tunnel action failed: " + err);
        }
        fetchTunnels(false);
    } catch (e) { }
}

async function deleteTunnel(id) {
    if (!confirm("Are you sure you want to delete this tunnel?")) return;
    try {
        await fetch(`/api/tunnels/${id}`, { method: 'DELETE' });
        fetchTunnels(false);
    } catch (e) { }
}

// ====================
// WIZARD MODAL LOGIC
// ====================
let currentStep = 1;

function goToStep(step) {
    if (step === 2) {
        // Validate Step 1 fields
        const name = document.getElementById('tunName').value.trim();
        const localPort = document.getElementById('tunLocalPort').value;
        const remotePort = document.getElementById('tunRemotePort').value;
        const remoteHost = document.getElementById('tunRemoteHost').value.trim();

        if (!name || !localPort || !remotePort || !remoteHost) {
            alert("Por favor completa todos los campos de la Etapa 1 antes de continuar.");
            return;
        }

        currentStep = 2;
        document.getElementById('wizardStep1').style.display = 'none';
        document.getElementById('wizardStep2').style.display = 'block';

        document.getElementById('stepDot1').style.color = 'var(--text-muted)';
        document.getElementById('stepDot1').querySelector('span').style.background = 'rgba(255,255,255,0.1)';
        document.getElementById('stepDot1').querySelector('span').style.color = 'var(--text-muted)';

        document.getElementById('stepDot2').style.color = 'var(--primary)';
        document.getElementById('stepDot2').querySelector('span').style.background = 'var(--primary)';
        document.getElementById('stepDot2').querySelector('span').style.color = '#000';
    } else {
        currentStep = 1;
        document.getElementById('wizardStep1').style.display = 'block';
        document.getElementById('wizardStep2').style.display = 'none';

        document.getElementById('stepDot1').style.color = 'var(--primary)';
        document.getElementById('stepDot1').querySelector('span').style.background = 'var(--primary)';
        document.getElementById('stepDot1').querySelector('span').style.color = '#000';

        document.getElementById('stepDot2').style.color = 'var(--text-muted)';
        document.getElementById('stepDot2').querySelector('span').style.background = 'rgba(255,255,255,0.1)';
        document.getElementById('stepDot2').querySelector('span').style.color = 'var(--text-muted)';
    }
}

function toggleTargetMethod() {
    const isId = document.getElementById('targetMethodId').checked;
    document.getElementById('groupTargetId').style.display = isId ? 'block' : 'none';
    document.getElementById('groupTargetTag').style.display = isId ? 'none' : 'block';
}

function populateProfileSelect(selectedProfile = '') {
    const select = document.getElementById('tunAwsProfileSelect');
    select.innerHTML = '';

    // Find unique AWS profiles from existing tunnels
    const knownProfiles = new Set();
    (tunnelsData || []).forEach(t => {
        if (t && t.type === 'aws-ssm' && t.aws_profile && t.aws_profile.trim()) {
            knownProfiles.add(t.aws_profile.trim());
        }
    });

    if (knownProfiles.size > 0) {
        knownProfiles.forEach(p => {
            const opt = document.createElement('option');
            opt.value = p;
            opt.innerText = `⚙️ Perfil existente: ${p}`;
            select.appendChild(opt);
        });
    }

    const newOpt = document.createElement('option');
    newOpt.value = '__NEW__';
    newOpt.innerText = '➕ Configurar un perfil nuevo...';
    select.appendChild(newOpt);

    if (selectedProfile && knownProfiles.has(selectedProfile)) {
        select.value = selectedProfile;
    } else {
        select.value = '__NEW__';
    }

    onProfileSelectChange();
}

function onProfileSelectChange() {
    const val = document.getElementById('tunAwsProfileSelect').value;
    const boxNew = document.getElementById('boxNewProfile');
    if (val === '__NEW__') {
        boxNew.style.display = 'block';
    } else {
        boxNew.style.display = 'none';
    }
}

function toggleTunnelFields() {
    const type = document.getElementById('tunType').value;
    document.getElementById('fields-aws-ssm').style.display = (type === 'aws-ssm') ? 'block' : 'none';
    document.getElementById('fields-ssh').style.display = (type === 'ssh') ? 'block' : 'none';

    if (type === 'aws-ssm') {
        toggleTargetMethod();
        onProfileSelectChange();
        toggleSSOFields();
    }
}

function toggleSSOFields() {
    const isSSO = document.getElementById('tunIsSSO').checked;
    document.getElementById('fields-ssm-sso').style.display = isSSO ? 'block' : 'none';
    document.getElementById('fields-ssm-static').style.display = isSSO ? 'none' : 'block';
}

function openTunnelModal() {
    document.getElementById('tunnelForm').reset();
    document.getElementById('tunnelId').value = "";
    document.getElementById('modalTitle').innerText = "Add Tunnel";
    document.getElementById('targetMethodId').checked = true;
    document.getElementById('tunIsSSO').checked = true;
    populateProfileSelect('');
    goToStep(1);
    toggleTunnelFields();
    document.getElementById('tunnelModal').classList.add('show');
}

function closeTunnelModal() {
    document.getElementById('tunnelModal').classList.remove('show');
}

function editTunnel(id) {
    const t = tunnelsData.find(x => x.id === id);
    if (!t) return;

    document.getElementById('modalTitle').innerText = "Edit Tunnel";
    document.getElementById('tunnelId').value = t.id;
    document.getElementById('tunName').value = t.name;
    document.getElementById('tunLocalPort').value = t.local_port;
    document.getElementById('tunRemotePort').value = t.remote_port;
    document.getElementById('tunRemoteHost').value = t.remote_host;

    document.getElementById('tunType').value = t.type;

    if (t.type === 'aws-ssm') {
        if (t.target && t.target.trim()) {
            document.getElementById('targetMethodId').checked = true;
            document.getElementById('tunTarget').value = t.target;
        } else if (t.target_tags && Object.keys(t.target_tags).length > 0) {
            document.getElementById('targetMethodTag').checked = true;
            document.getElementById('tunTargetTags').value = Object.entries(t.target_tags).map(([k, v]) => `${k}=${v}`).join(",");
        } else {
            document.getElementById('targetMethodId').checked = true;
        }

        populateProfileSelect(t.aws_profile || '');
        document.getElementById('tunAwsProfile').value = t.aws_profile || "";

        document.getElementById('tunIsSSO').checked = (t.is_sso !== false);
        document.getElementById('tunSSOStartURL').value = t.sso_start_url || "";
        document.getElementById('tunSSORegion').value = t.sso_region || "";
        document.getElementById('tunRegionSSO').value = t.region || "";
        document.getElementById('tunSSOAccountID').value = t.sso_account_id || "";
        document.getElementById('tunSSORoleName').value = t.sso_role_name || "";

        document.getElementById('tunRegionStatic').value = t.region || "";
        document.getElementById('tunAccessKey').value = t.aws_access_key_id || "";
        document.getElementById('tunSecretKey').value = t.aws_secret_access_key || "";
    } else if (t.type === 'ssh') {
        document.getElementById('tunJumpHost').value = t.jump_host || "";
        document.getElementById('tunJumpUser').value = t.jump_user || "";
        document.getElementById('tunSSHKey').value = t.ssh_key_file || "";
    }

    goToStep(1);
    toggleTunnelFields();
    document.getElementById('tunnelModal').classList.add('show');
}

async function saveTunnel(e) {
    e.preventDefault();

    const id = document.getElementById('tunnelId').value || crypto.randomUUID();
    const type = document.getElementById('tunType').value;

    const payload = {
        id: id,
        name: document.getElementById('tunName').value.trim(),
        type: type,
        remote_port: parseInt(document.getElementById('tunRemotePort').value),
        local_port: parseInt(document.getElementById('tunLocalPort').value),
        remote_host: document.getElementById('tunRemoteHost').value.trim(),
    };

    if (type === 'aws-ssm') {
        const targetMethod = document.querySelector('input[name="targetMethod"]:checked').value;
        if (targetMethod === 'id') {
            payload.target = document.getElementById('tunTarget').value.trim();
        } else {
            const tagStr = document.getElementById('tunTargetTags').value.trim();
            if (tagStr) {
                payload.target_tags = {};
                tagStr.split(',').forEach(kv => {
                    const parts = kv.split('=');
                    if (parts.length === 2) payload.target_tags[parts[0].trim()] = parts[1].trim();
                });
            }
        }

        const profileSelect = document.getElementById('tunAwsProfileSelect').value;
        if (profileSelect !== '__NEW__') {
            // Reusing existing profile!
            payload.aws_profile = profileSelect;
            const existing = tunnelsData.find(x => x.aws_profile === profileSelect && x.type === 'aws-ssm');
            if (existing) {
                payload.is_sso = existing.is_sso;
                payload.sso_start_url = existing.sso_start_url;
                payload.sso_region = existing.sso_region;
                payload.sso_account_id = existing.sso_account_id;
                payload.sso_role_name = existing.sso_role_name;
                payload.region = existing.region;
                payload.aws_access_key_id = existing.aws_access_key_id;
                payload.aws_secret_access_key = existing.aws_secret_access_key;
            }
        } else {
            // New profile configuration
            payload.aws_profile = document.getElementById('tunAwsProfile').value.trim() || "default";
            const isSSO = document.getElementById('tunIsSSO').checked;
            payload.is_sso = isSSO;

            if (isSSO) {
                payload.sso_start_url = document.getElementById('tunSSOStartURL').value.trim();
                payload.sso_region = document.getElementById('tunSSORegion').value.trim();
                payload.region = document.getElementById('tunRegionSSO').value.trim();
                payload.sso_account_id = document.getElementById('tunSSOAccountID').value.trim();
                payload.sso_role_name = document.getElementById('tunSSORoleName').value.trim();
            } else {
                payload.region = document.getElementById('tunRegionStatic').value.trim();
                payload.aws_access_key_id = document.getElementById('tunAccessKey').value.trim();
                payload.aws_secret_access_key = document.getElementById('tunSecretKey').value.trim();
            }
        }
    } else if (type === 'ssh') {
        payload.jump_host = document.getElementById('tunJumpHost').value.trim();
        payload.jump_user = document.getElementById('tunJumpUser').value.trim();
        payload.ssh_key_file = document.getElementById('tunSSHKey').value.trim();
    }

    const isEdit = !!document.getElementById('tunnelId').value;
    const url = isEdit ? `/api/tunnels/${id}` : `/api/tunnels`;
    const method = isEdit ? `PUT` : `POST`;

    try {
        const res = await fetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (res.ok) {
            closeTunnelModal();
            fetchTunnels(true);
        } else {
            const err = await res.text();
            alert("Error saving tunnel: " + err);
        }
    } catch (e) {
        alert("Error connecting to server.");
    }
}

// Init
updateStatus();
fetchProxies();
fetchTunnels(true);

// Periodic polling for status updates (including SSO verification flow)
setInterval(() => {
    if (currentTab === 'tunnels') {
        fetchTunnels(false);
    }
}, 3000);
